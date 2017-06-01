// Copyright 2017 Google Inc. All Rights Reserved.
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

package keys

import (
	"context"
	"crypto"
	"fmt"

	"github.com/golang/protobuf/ptypes"
	"github.com/golang/protobuf/ptypes/any"
	"github.com/google/trillian/crypto/keyspb"
)

// PEMSignerFactory handles PEM-encoded private keys.
// It implements keys.SignerFactory.
type PEMSignerFactory struct{}

// NewSigner uses the information in pb to return a crypto.Signer.
// pb must be one of the following types:
// - keyspb.PEMKeyFile
// - keyspb.PrivateKey
func (f PEMSignerFactory) NewSigner(ctx context.Context, pb *any.Any) (crypto.Signer, error) {
	var privateKey ptypes.DynamicAny
	if err := ptypes.UnmarshalAny(pb, &privateKey); err != nil {
		return nil, fmt.Errorf("failed to unmarshal private key: %v", err)
	}

	switch privateKey := privateKey.Message.(type) {
	case *keyspb.PEMKeyFile:
		return NewFromPrivatePEMFile(privateKey.GetPath(), privateKey.GetPassword())
	case *keyspb.PrivateKey:
		return NewFromPrivateDER(privateKey.GetDer())
	}

	return nil, fmt.Errorf("unsupported PrivateKey type: %T", privateKey.Message)
}

// Generate creates a new private key based on a key specification.
// It returns a proto that can be passed to NewSigner() to get a crypto.Signer.
func (f PEMSignerFactory) Generate(ctx context.Context, spec *keyspb.Specification) (*any.Any, error) {
	key, err := NewFromSpec(spec)
	if err != nil {
		return nil, fmt.Errorf("error generating key: %v", err)
	}

	der, err := MarshalPrivateKey(key)
	if err != nil {
		return nil, fmt.Errorf("error marshaling private key as DER: %v", err)
	}

	return ptypes.MarshalAny(&keyspb.PrivateKey{Der: der})
}
