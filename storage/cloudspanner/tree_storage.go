// Copyright 2018 Google Inc. All Rights Reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package cloudspanner

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"cloud.google.com/go/spanner"
	"github.com/golang/glog"
	"github.com/golang/protobuf/proto"
	"github.com/golang/protobuf/ptypes/any"
	"github.com/google/trillian"
	"github.com/google/trillian/storage"
	"github.com/google/trillian/storage/cache"
	"github.com/google/trillian/storage/cloudspanner/spannerpb"
	"github.com/google/trillian/storage/storagepb"
	"github.com/google/trillian/trees"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

var (
	// ErrNotFound is returned when a read/lookup fails because there was no such
	// item.
	ErrNotFound = status.Errorf(codes.NotFound, "not found")

	// ErrNotImplemented is returned by any interface methods which have not been
	// implemented yet.
	ErrNotImplemented = errors.New("not implemented")

	// ErrTransactionClosed is returned by interface methods when an operation is
	// attempted on a transaction whose Commit or Rollback methods have
	// previously been called.
	ErrTransactionClosed = errors.New("transaction is closed")

	// ErrWrongTXType is returned when, somehow, a write operation is attempted
	// with a read-only transaction.  This should not even be possible.
	ErrWrongTXType = errors.New("mutating method called on read-only transaction")

	// errFinished is only used to terminate reads early, once all required data
	// has been read. It should never be returned to a caller.
	errFinished = errors.New("read complete")
)

const (
	subtreeTbl   = "SubtreeData"
	colBucket    = "Bucket"
	colSubtree   = "Subtree"
	colSubtreeID = "SubtreeID"
	colTreeID    = "TreeID"
	colRevision  = "Revision"
)

// treeStorage provides a shared base for the concrete CloudSpanner-backed
// implementation of the Trillian storage.LogStorage and storage.MapStorage
// interfaces.
type treeStorage struct {
	admin  storage.AdminStorage
	opts   TreeStorageOptions
	client *spanner.Client
}

// TreeStorageOptions holds various levers for configuring the tree storage instance.
type TreeStorageOptions struct {
	// ReadOnlyStaleness controls how far in the past a read-only snapshot
	// transaction will read.
	// This is intended to allow Spanner to use local replicas for read requests
	// to help with performance.
	// See https://cloud.google.com/spanner/docs/timestamp-bounds for more details.
	ReadOnlyStaleness time.Duration
}

func newTreeStorageWithOpts(client *spanner.Client, opts TreeStorageOptions) *treeStorage {
	return &treeStorage{client: client, admin: nil, opts: opts}
}

type spanRead interface {
	Query(context.Context, spanner.Statement) *spanner.RowIterator
	Read(ctx context.Context, table string, keys spanner.KeySet, columns []string) *spanner.RowIterator
	ReadUsingIndex(ctx context.Context, table, index string, keys spanner.KeySet, columns []string) *spanner.RowIterator
	ReadRow(ctx context.Context, table string, key spanner.Key, columns []string) (*spanner.Row, error)
}

// latestSTH reads and returns the newest STH.
func (t *treeStorage) latestSTH(ctx context.Context, stx spanRead, treeID int64) (*spannerpb.TreeHead, error) {
	query := spanner.NewStatement(
		"SELECT t.TreeID, t.TimestampNanos, t.TreeSize, t.RootHash, t.RootSignature, t.TreeRevision, t.TreeMetadata FROM TreeHeads t" +
			"   WHERE t.TreeID = @tree_id" +
			"   ORDER BY t.TimestampNanos DESC " +
			"   LIMIT 1")
	query.Params["tree_id"] = treeID

	var th *spannerpb.TreeHead
	rows := stx.Query(ctx, query)
	defer rows.Stop()
	err := rows.Do(func(r *spanner.Row) error {
		tth := &spannerpb.TreeHead{}
		var sig, meta []byte
		if err := r.Columns(&tth.TreeId, &tth.TsNanos, &tth.TreeSize, &tth.RootHash, &sig, &tth.TreeRevision, &meta); err != nil {
			return err
		}
		sigPB := &spannerpb.DigitallySigned{}
		if err := proto.Unmarshal(sig, sigPB); err != nil {
			return err
		}
		tth.Signature = sigPB

		metaPB := &any.Any{}
		if err := proto.Unmarshal(meta, metaPB); err != nil {
			return err
		}
		tth.Metadata = metaPB

		th = tth
		return nil
	})
	if err != nil {
		return nil, err
	}
	if th == nil {
		glog.Warningf("no head found for treeID %v", treeID)
		return nil, storage.ErrTreeNeedsInit
	}
	return th, nil
}

type newCacheFn func(*trillian.Tree) (cache.SubtreeCache, error)

func (t *treeStorage) getTreeAndConfig(ctx context.Context, treeID int64, opts trees.GetOpts) (*trillian.Tree, proto.Message, error) {
	tree, err := trees.GetTree(ctx, t.admin, treeID, opts)
	if err != nil {
		return nil, nil, err
	}
	config, err := unmarshalSettings(tree)
	if err != nil {
		return nil, nil, err
	}
	return tree, config, nil
}

// begin returns a newly started tree transaction for the specified tree.
func (t *treeStorage) begin(ctx context.Context, treeID int64, opts trees.GetOpts, newCache newCacheFn, stx spanRead) (*treeTX, error) {
	tree, config, err := t.getTreeAndConfig(ctx, treeID, opts)
	if err != nil {
		return nil, err
	}
	cache, err := newCache(tree)
	if err != nil {
		return nil, err
	}
	treeTX := &treeTX{
		treeID: treeID,
		ts:     t,
		stx:    stx,
		cache:  cache,
		config: config,
	}

	return treeTX, nil
}

// getLatestRoot populates this TX with the newest tree root visible (when
// taking read-staleness into account) by this transaction.
func (t *treeTX) getLatestRoot(ctx context.Context) error {
	t.getLatestRootOnce.Do(func() {
		t._currentSTH, t._currentSTHErr = t.ts.latestSTH(ctx, t.stx, t.treeID)
		if t._currentSTH != nil {
			t._writeRev = t._currentSTH.TreeRevision + 1
		}
	})

	return t._currentSTHErr
}

// treeTX is a concrete implementation of the Trillian
// storage.TreeTX interface.
type treeTX struct {
	treeID int64

	ts *treeStorage

	// mu guards the nil setting/checking of stx as part of the open checking.
	mu sync.RWMutex
	// stx is the underlying Spanner transaction in which all operations will be
	// performed.
	stx spanRead

	// config holds the StorageSettings proto acquired from the trillian.Tree.
	// Varies according to tree_type (LogStorageConfig vs MapStorageConfig).
	config proto.Message

	// currentSTH holds a copy of the latest known STH at the time the
	// transaction was started, or nil if there was no STH.
	_currentSTH    *spannerpb.TreeHead
	_currentSTHErr error

	// writeRev is the tree revision at which any writes will be made.
	_writeRev int64

	cache cache.SubtreeCache

	getLatestRootOnce sync.Once
}

func (t *treeTX) currentSTH(ctx context.Context) (*spannerpb.TreeHead, error) {
	if err := t.getLatestRoot(ctx); err != nil {
		return nil, err
	}
	return t._currentSTH, nil
}

func (t *treeTX) writeRev(ctx context.Context) (int64, error) {
	if err := t.getLatestRoot(ctx); err != nil {
		return -1, err
	}
	return t._writeRev, nil
}

// storeSubtrees adds buffered writes to the in-flight transaction to store the
// passed in subtrees.
func (t *treeTX) storeSubtrees(sts []*storagepb.SubtreeProto) error {
	stx, ok := t.stx.(*spanner.ReadWriteTransaction)
	if !ok {
		return ErrWrongTXType
	}
	for _, st := range sts {
		if st == nil {
			continue
		}
		stBytes, err := proto.Marshal(st)
		if err != nil {
			return err
		}
		m := spanner.Insert(
			subtreeTbl,
			[]string{colTreeID, colSubtreeID, colRevision, colSubtree},
			[]interface{}{t.treeID, st.Prefix, t._writeRev, stBytes},
		)
		if err := stx.BufferWrite([]*spanner.Mutation{m}); err != nil {
			return err
		}
	}
	return nil
}

func (t *treeTX) flushSubtrees() error {
	return t.cache.Flush(t.storeSubtrees)
}

// Commit attempts to apply all actions perfomed to the underlying Spanner
// transaction.  If this call returns an error, any values READ via this
// transaction MUST NOT be used.
// On return from the call, this transaction will be in a closed state.
func (t *treeTX) Commit() error {
	t.mu.Lock()
	defer func() {
		t.stx = nil
		t.mu.Unlock()
	}()

	if t.stx == nil {
		return ErrTransactionClosed
	}
	switch stx := t.stx.(type) {
	case *spanner.ReadOnlyTransaction:
		glog.V(1).Infof("Closed readonly tx %p", stx)
		stx.Close()
		return nil
	case *spanner.ReadWriteTransaction:
		return t.flushSubtrees()
	default:
		return fmt.Errorf("internal error: unknown transaction type %T", stx)
	}
}

// Rollback aborts any operations perfomed on the underlying Spanner
// transaction.
// On return from the call, this transaction will be in a closed state.
func (t *treeTX) Rollback() error {
	t.mu.Lock()
	defer func() {
		t.stx = nil
		t.mu.Unlock()
	}()

	if t.stx == nil {
		return ErrTransactionClosed
	}
	return nil
}

func (t *treeTX) Close() error {
	if t.IsOpen() {
		if err := t.Rollback(); err != nil && err != ErrTransactionClosed {
			glog.Warningf("Rollback error on Close(): %v", err)
			return err
		}
	}
	return nil
}

// IsOpen returns true iff neither Commit nor Rollback have been called.
// If this function returns false, further operations may not be attempted on
// this transaction object.
func (t *treeTX) IsOpen() bool {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return t.stx != nil
}

// ReadRevision returns the tree revision at which the currently visible (taking
// into account read-staleness) STH was stored.
func (t *treeTX) ReadRevision() int64 {
	sth, err := t.currentSTH(context.TODO())
	if err != nil {
		panic(err)
	}
	return sth.TreeRevision
}

// WriteRevision returns the tree revision at which any tree-modifying
// operations will write.
func (t *treeTX) WriteRevision() int64 {
	rev, err := t.writeRev(context.TODO())
	if err != nil {
		panic(err)
	}
	return rev
}

// nodeIDToKey returns a []byte suitable for use as a primary key column for
// the subtree which contains the id.
// If id's prefix is not byte-aligned, an error will be returned.
func subtreeKey(id storage.NodeID) ([]byte, error) {
	// TODO(al): extend this check to ensure id is at a tree stratum boundary.
	if id.PrefixLenBits%8 != 0 {
		return nil, fmt.Errorf("id.PrefixLenBits (%d) is not a multiple of 8; it cannot be a subtree prefix", id.PrefixLenBits)
	}
	return id.Path[:id.PrefixLenBits/8], nil
}

// getSubtree retrieves the most recent subtree specified by id at (or below)
// the requested revision.
// If no such subtree exists it returns nil.
func (t *treeTX) getSubtree(ctx context.Context, rev int64, id storage.NodeID) (p *storagepb.SubtreeProto, e error) {
	stID, err := subtreeKey(id)
	if err != nil {
		return nil, err
	}

	var ret *storagepb.SubtreeProto
	prefix := spanner.Key{t.treeID, stID}.AsPrefix()
	rows := t.stx.Read(ctx, subtreeTbl, prefix, []string{colRevision, colSubtree})
	err = rows.Do(func(r *spanner.Row) error {
		var rRev int64
		var st storagepb.SubtreeProto
		stBytes := make([]byte, 1<<20)
		if err = r.Columns(&rRev, &stBytes); err != nil {
			return err
		}
		if err = proto.Unmarshal(stBytes, &st); err != nil {
			return err
		}

		if rRev > rev {
			// Too new, skip this row and wait for the next.
			return nil
		}
		if got, want := stID, st.Prefix; !bytes.Equal(got, want) {
			return fmt.Errorf("got subtree with prefix %v, wanted %v", got, want)
		}
		if got, want := rRev, rev; got > rev {
			return fmt.Errorf("got subtree rev %d, wanted <= %d", got, want)
		}
		ret = &st

		// If this is a subtree with a zero-length prefix, we'll need to create an
		// empty Prefix field:
		if st.Prefix == nil && len(stID) == 0 {
			st.Prefix = []byte{}
		}
		// We've got what we want, tell spanner to stop reading by returning
		// not-really-an-error:
		return errFinished
	})
	if err == errFinished {
		err = nil
	}
	return ret, err
}

// GetMerkleNodes returns the requested set of nodes at, or before, the
// specified tree revision.
func (t *treeTX) GetMerkleNodes(ctx context.Context, rev int64, ids []storage.NodeID) ([]storage.Node, error) {
	t.mu.RLock()
	defer t.mu.RUnlock()
	if t.stx == nil {
		return nil, ErrTransactionClosed
	}

	return t.cache.GetNodes(ids,
		func(ids []storage.NodeID) ([]*storagepb.SubtreeProto, error) {
			// Request the various subtrees in parallel.
			// c will carry any retrieved subtrees
			c := make(chan *storagepb.SubtreeProto, len(ids))
			// err will carry any errors encountered while reading from spanner,
			// although we'll only return to the caller the first one (if indeed
			// there are any).
			errc := make(chan error, len(ids))

			// Spawn goroutines for each request
			for _, id := range ids {
				id := id
				go func() {
					st, err := t.getSubtree(ctx, rev, id)
					if err != nil {
						errc <- err
						return
					}
					c <- st
				}()
			}

			// Now wait for the goroutines to signal their completion, and collect
			// the results.
			ret := make([]*storagepb.SubtreeProto, 0, len(ids))
			for range ids {
				select {
				case err := <-errc:
					return nil, err
				case st := <-c:
					if st != nil {
						ret = append(ret, st)
					}
				}
			}
			return ret, nil
		})
}

// SetMerkleNodes stores the provided merkle nodes at the writeRevision of the
// transaction.
func (t *treeTX) SetMerkleNodes(ctx context.Context, nodes []storage.Node) error {
	t.mu.RLock()
	defer t.mu.RUnlock()
	if t.stx == nil {
		return ErrTransactionClosed
	}

	writeRev, err := t.writeRev(ctx)
	if err != nil {
		return err
	}

	for _, n := range nodes {
		err := t.cache.SetNodeHash(
			n.NodeID,
			n.Hash,
			func(nID storage.NodeID) (*storagepb.SubtreeProto, error) {
				return t.getSubtree(ctx, writeRev-1, nID)
			})
		if err != nil {
			return err
		}
	}
	return nil
}

func checkDatabaseAccessible(ctx context.Context, client *spanner.Client) error {
	stmt := spanner.NewStatement("SELECT 1")
	// We don't care about freshness here, being able to read *something* is enough
	rows := client.Single().Query(ctx, stmt)
	defer rows.Stop()
	return rows.Do(func(row *spanner.Row) error { return nil })
}

// snapshotTX provides the standard methods for snapshot-based TXs.
type snapshotTX struct {
	client *spanner.Client

	// mu guards stx, which is set to nil when the TX is closed.
	mu  sync.RWMutex
	stx spanRead
	ls  *logStorage
}

func (t *snapshotTX) Commit() error {
	// No work required to commit snapshot transactions
	return t.Close()
}

func (t *snapshotTX) Rollback() error {
	return t.Close()
}

func (t *snapshotTX) Close() error {
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.stx == nil {
		return ErrTransactionClosed
	}
	if stx, ok := t.stx.(*spanner.ReadOnlyTransaction); ok {
		glog.Infof("Closed log snapshot %p", stx)
		stx.Close()
	}
	t.stx = nil

	return nil
}
