// Unstable Build LLC ("COMPANY") CONFIDENTIAL
//
// Unpublished Copyright (c) 2023-2024 Unstable Build, All Rights Reserved.
//
// NOTICE: All information contained herein is, and remains the property of COMPANY.
// The intellectual and technical concepts contained herein are proprietary to
// COMPANY and may be covered by U.S. and Foreign Patents, patents in process,
// and are protected by trade secret or copyright law. Dissemination of this information
// or reproduction of this material is strictly forbidden unless prior written permission
// is obtained from COMPANY. Access to the source code contained herein is hereby
// forbidden to anyone except current COMPANY employees, managers or contractors who
// have executed Confidentiality and Non-disclosure agreements explicitly covering such access.
//
// The copyright notice above does not evidence any actual or intended publication or
// disclosure of this source code, which includes information that is confidential and/or
// proprietary, and is a trade secret, of COMPANY. ANY REPRODUCTION, MODIFICATION,
// DISTRIBUTION, PUBLIC  PERFORMANCE, OR PUBLIC DISPLAY OF OR THROUGH USE OF THIS SOURCE CODE
// WITHOUT  THE EXPRESS WRITTEN CONSENT OF COMPANY IS STRICTLY PROHIBITED, AND IN
// VIOLATION OF APPLICABLE LAWS AND INTERNATIONAL TREATIES. THE RECEIPT OR POSSESSION OF
// THIS SOURCE CODE AND/OR RELATED INFORMATION DOES NOT CONVEY OR IMPLY ANY RIGHTS TO
// REPRODUCE, DISCLOSE OR DISTRIBUTE ITS CONTENTS, OR TO MANUFACTURE, USE, OR SELL
// ANYTHING THAT IT MAY DESCRIBE, IN WHOLE OR IN PART.

package oxapi

import (
	"fmt"
	"net/http"
	"net/url"
	"time"

	"github.com/ernestrc/go-multierror"
	"github.com/julienschmidt/httprouter"
	log "github.com/sirupsen/logrus"
	blueauth "github.com/unstablebuild/blue/auth"
	"github.com/unstablebuild/blue/document"
	"github.com/unstablebuild/blue/logging"
	"github.com/unstablebuild/blue/logging/trace"
	"github.com/unstablebuild/blue/release"
	"github.com/unstablebuild/blue/release/gcsrelease"
	"unstable.build/go-tui/cmd/rune/api"
	"unstable.build/go-tui/cmd/rune/api/account"
	"unstable.build/go-tui/cmd/rune/api/user"
	"unstable.build/go-tui/cmd/rune/auth"
	"unstable.build/go-tui/localstorage/bluestore"
)

// HTTPAPI serves the ox-api HTTP surface, including website and release routes.
type HTTPAPI struct {
	*http.ServeMux
	accStore account.Store

	// used for health response
	keys blueauth.Keys
	svc  document.Service
}

// AccountStore returns the account store used by the HTTP API.
func (a HTTPAPI) AccountStore() account.Store {
	return a.accStore
}

func (a HTTPAPI) serveHealth(
	w http.ResponseWriter, r *http.Request,
) {
	const healthCallType = "health"
	traceID, ctx := trace.FromContextOrNew(r.Context())
	fields := []logging.Field{
		{Key: logging.KeyClass, Value: "ox-api"},
		{Key: "Method", Value: r.Method},
		{Key: "URL", Value: r.URL.String()},
	}
	attemptAt := logging.LogAttempt(traceID, healthCallType, fields...)

	var ret error
	if _, err := a.keys.Verify(ctx); err != nil {
		ret = multierror.Append(ret, err)
	}
	if it, err := a.svc.List(ctx, nil); err != nil {
		ret = multierror.Append(ret, err)
	} else if err := it.Close(); err != nil {
		ret = multierror.Append(ret, err)
	}

	logging.LogResultInfo(ret, attemptAt, traceID, healthCallType, fields...)
	if ret != nil {
		w.WriteHeader(http.StatusInternalServerError)
	} else {
		w.WriteHeader(http.StatusOK)
	}
}

func (a HTTPAPI) serveHealthRouter(
	w http.ResponseWriter, r *http.Request, params httprouter.Params,
) {
	a.serveHealth(w, r)
}

// Auth0SecretID is the secret identifier used to initialize the Auth0 store.
const Auth0SecretID = "auth0-ox-api-prod"

// NewHTTPApi constructs the ox-api HTTP handler stack.
func NewHTTPApi(
	accDB document.Service,
	signKeys, verifyKeys blueauth.Keys,
	secretStore blueauth.SecretStore,
	tokenExpiry time.Duration,
	signupURL *url.URL,
	apiURL *url.URL,
	authConfig auth.Config,
	releaseManager release.Manager,
	releaseSigner gcsrelease.Signer,
	rpcAuthorizer blueauth.Authorizer[auth.RPCUser],
) (HTTPAPI, error) {
	if releaseManager == nil {
		panic("oxapi.NewHTTPApi: releaseManager must not be nil")
	}
	if releaseSigner == nil {
		panic("oxapi.NewHTTPApi: releaseSigner must not be nil")
	}
	ret := HTTPAPI{
		ServeMux: http.NewServeMux(),
		svc:      accDB,
		keys:     verifyKeys,
	}
	logger := log.StandardLogger()

	cfgHandler, err := auth.ServeNativeConfig(logger, apiURL)
	if err != nil {
		return HTTPAPI{}, fmt.Errorf("serve config: %v", err)
	}
	ret.Handle(auth.ServeConfigPath, cfgHandler)

	userStore, err := user.NewAuth0Store(secretStore, Auth0SecretID, authConfig)
	if err != nil {
		return HTTPAPI{}, fmt.Errorf("new auth0 service: %v", err)
	}

	rpcGranter := newGranter(userStore, signupURL.String())
	tokenHandler := blueauth.TokenHTTPHandler(signKeys, secretStore, rpcGranter, tokenExpiry)
	ret.Handle(auth.ServeTokenPath, tokenHandler)

	ret.Handle("/telemetry", NewTelemetryHandler(logger, verifyKeys))

	ret.HandleFunc("/health", ret.serveHealth)

	accStore := account.NewDocumentStore(bluestore.AdaptTo(accDB), userStore)
	// There are two different authorizers, and so two different oauth2 tokens used
	// by the rune infrastructure: one authorizer for the http api, which uses the data
	// extracted from an oauth2 provider signed token, to grant access to the web api,
	// for use by the website only.
	// The other authorizer, is employed by the rpc api, which actually grant access
	// to functionality serviced to rune clients, and uses the token
	// signed by the /o/oauth2/token http endpoint above.
	// Therefore, we need a separate authorizer for web endpoints (than for rpc endpoints).
	webAuthorizer := WebAuthorizer(accStore)

	middleWareConfig := blueauth.MiddlewareConfig[auth.WebUser]{
		VerifyKeys: verifyKeys,
		Authorizer: webAuthorizer,
	}

	router := httprouter.New()
	router.GET("/api/health", ret.serveHealthRouter)
	api.AddAccountHTTPHandles(router, accStore)
	api.AddUserHTTPHandles(router, accStore, userStore)

	var securedHandler http.Handler = router
	securedHandler = blueauth.WithMiddleware(securedHandler, middleWareConfig)
	securedHandler = withCORS(securedHandler)
	ret.Handle("/api/", securedHandler)
	ret.accStore = accStore

	// Mount release HTTP API with RPC-style auth (for rune clients).
	releaseHandler := gcsrelease.NewHandler(releaseManager, releaseSigner)
	rpcMiddlewareConfig := blueauth.MiddlewareConfig[auth.RPCUser]{
		VerifyKeys: verifyKeys,
		Authorizer: rpcAuthorizer,
	}
	var securedReleaseHandler = http.StripPrefix("/api/releases", releaseHandler)
	securedReleaseHandler = blueauth.WithMiddleware(securedReleaseHandler, rpcMiddlewareConfig)
	ret.Handle("/api/releases/", securedReleaseHandler)

	return ret, nil
}
