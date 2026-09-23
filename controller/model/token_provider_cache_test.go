package model

import (
	"crypto/x509"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/hanzozt/zt/v2/controller/db"
	cmap "github.com/orcaman/concurrent-map/v2"
	"github.com/stretchr/testify/require"
)

func TestVerifyTokenByInspectionAudience(t *testing.T) {
	ca := newRootCa()
	leaf := ca.NewLeafWithAKID()
	key, err := newKey(leaf.cert, []*x509.Certificate{leaf.cert, ca.cert})
	require.NoError(t, err)
	resolver, err := newTestJwksResolver()
	require.NoError(t, err)
	resolver.AddKey(key, leaf.key)

	issuer, endpoint, claim := "https://hanzo.id", "https://hanzo.id/v1/iam/.well-known/jwks", "sub"
	signer := &db.ExternalJwtSigner{Name: "iam", Issuer: &issuer, JwksEndpoint: &endpoint, IdentityIdClaimsSelector: &claim, Enabled: true}
	cache := &TokenIssuerCache{issuers: cmap.New[TokenIssuer]()}
	cache.issuers.Set(issuer, &TokenIssuerExtJwt{externalJwtSigner: signer, jwksResolver: resolver, kidToPubKey: map[string]IssuerPublicKey{}})

	token := jwt.NewWithClaims(jwt.SigningMethodRS256, jwt.RegisteredClaims{
		Issuer:    issuer,
		Subject:   "admin/hanzo-cloud",
		Audience:  jwt.ClaimStrings{"hanzo-cloud"},
		ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Minute)),
	})
	token.Header["kid"] = key.KeyId
	signed, err := token.SignedString(leaf.key)
	require.NoError(t, err)

	t.Run("a signer without an audience accepts any", func(t *testing.T) {
		result, err := cache.VerifyTokenByInspection(signed)
		require.NoError(t, err)
		require.Equal(t, "admin/hanzo-cloud", result.IdClaimValue)
	})

	t.Run("a signer with an audience requires it", func(t *testing.T) {
		audience := "hanzo-cli"
		signer.Audience = &audience
		_, err := cache.VerifyTokenByInspection(signed)
		require.Error(t, err)
	})
}
