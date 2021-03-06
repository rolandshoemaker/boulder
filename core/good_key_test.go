// Copyright 2014 ISRG.  All rights reserved
// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package core

import (
	"crypto/ecdsa"
	"crypto/rand"
	"crypto/rsa"
	"math/big"
	"testing"

	"github.com/letsencrypt/boulder/test"
)

func TestUnknownKeyType(t *testing.T) {
	notAKey := struct{}{}
	test.Assert(t, !GoodKey(notAKey), "Should have rejeected a key of unknown type")
}

func TestWrongKeyType(t *testing.T) {
	ecdsaKey := ecdsa.PublicKey{}
	test.Assert(t, !GoodKey(&ecdsaKey), "Should have rejected ECDSA key.")
	test.Assert(t, !GoodKey(ecdsaKey), "Should have rejected ECDSA key.")
}

func TestSmallModulus(t *testing.T) {
	private, err := rsa.GenerateKey(rand.Reader, 2040)
	test.AssertNotError(t, err, "Error generating key")
	test.Assert(t, !GoodKey(&private.PublicKey), "Should have rejected too-short key.")
	test.Assert(t, !GoodKey(private.PublicKey), "Should have rejected too-short key.")
}

func TestSmallExponent(t *testing.T) {
	bigOne := big.NewInt(1)
	key := rsa.PublicKey{
		N: bigOne.Lsh(bigOne, 2048),
		E: 5,
	}
	test.Assert(t, !GoodKey(&key), "Should have rejected small exponent.")
}

func TestEvenExponent(t *testing.T) {
	bigOne := big.NewInt(1)
	key := rsa.PublicKey{
		N: bigOne.Lsh(bigOne, 2048),
		E: 1 << 17,
	}
	test.Assert(t, !GoodKey(&key), "Should have rejected even exponent.")
}

func TestEvenModulus(t *testing.T) {
	bigOne := big.NewInt(1)
	key := rsa.PublicKey{
		N: bigOne.Lsh(bigOne, 2048),
		E: (1 << 17) + 1,
	}
	test.Assert(t, !GoodKey(&key), "Should have rejected even modulus.")
}

func TestModulusDivisibleBy752(t *testing.T) {
	N := big.NewInt(1)
	N.Lsh(N, 2048)
	N.Add(N, big.NewInt(1))
	N.Mul(N, big.NewInt(751))
	key := rsa.PublicKey{
		N: N,
		E: (1 << 17) + 1,
	}
	test.Assert(t, !GoodKey(&key), "Should have rejected modulus divisible by 751.")
}

func TestGoodKey(t *testing.T) {
	private, err := rsa.GenerateKey(rand.Reader, 2048)
	test.AssertNotError(t, err, "Error generating key")
	test.Assert(t, GoodKey(&private.PublicKey), "Should have accepted good key.")
}
