// Copyright 2014 ISRG.  All rights reserved
// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package main

import (
	"fmt"
	"net/http"
	"os"
	"time"

	"github.com/letsencrypt/boulder/Godeps/_workspace/src/github.com/cactus/go-statsd-client/statsd"

	"github.com/letsencrypt/boulder/ca"
	"github.com/letsencrypt/boulder/cmd"
	blog "github.com/letsencrypt/boulder/log"
	"github.com/letsencrypt/boulder/ra"
	"github.com/letsencrypt/boulder/sa"
	"github.com/letsencrypt/boulder/va"
	"github.com/letsencrypt/boulder/wfe"
)

type timedHandler struct {
	f     func(w http.ResponseWriter, r *http.Request)
	stats statsd.Statter
}

var openConnections int64 = 0

func HandlerTimer(handler http.Handler, stats statsd.Statter) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		cStart := time.Now()
		openConnections += 1
		stats.Gauge("HttpConnectionsOpen", openConnections, 1.0)

		handler.ServeHTTP(w, r)

		openConnections -= 1
		stats.Gauge("HttpConnectionsOpen", openConnections, 1.0)

		// (FIX: this doesn't seem to really work at catching errors...)
		state := "Success"
		for _, h := range w.Header()["Content-Type"] {
			if h == "application/problem+json" {
				state = "Error"
				break
			}
		}
		// set resp timing key based on success / failure
		stats.TimingDuration(fmt.Sprintf("HttpResponseTime.%s.%s", r.URL, state), time.Since(cStart), 1.0)
	})
}

func main() {
	app := cmd.NewAppShell("boulder")
	app.Action = func(c cmd.Config) {
		stats, err := statsd.NewClient(c.Statsd.Server, c.Statsd.Prefix)
		cmd.FailOnError(err, "Couldn't connect to statsd")

		// Set up logging
		auditlogger, err := blog.Dial(c.Syslog.Network, c.Syslog.Server, c.Syslog.Tag, stats)
		cmd.FailOnError(err, "Could not connect to Syslog")

		// AUDIT[ Error Conditions ] 9cc4d537-8534-4970-8665-4b382abe82f3
		defer auditlogger.AuditPanic()

		blog.SetAuditLogger(auditlogger)

		// Run StatsD profiling
		go cmd.ProfileCmd("Monolith", stats)

		// Create the components
		wfei := wfe.NewWebFrontEndImpl()
		sa, err := sa.NewSQLStorageAuthority(c.SA.DBDriver, c.SA.DBName)
		cmd.FailOnError(err, "Unable to create SA")
		sa.SetSQLDebug(c.SQL.SQLDebug)

		ra := ra.NewRegistrationAuthorityImpl()

		va := va.NewValidationAuthorityImpl(c.CA.TestMode)

		cadb, err := ca.NewCertificateAuthorityDatabaseImpl(c.CA.DBDriver, c.CA.DBName)
		cmd.FailOnError(err, "Failed to create CA database")

		ca, err := ca.NewCertificateAuthorityImpl(cadb, c.CA)
		cmd.FailOnError(err, "Unable to create CA")

		if c.SQL.CreateTables {
			err = sa.CreateTablesIfNotExists()
			cmd.FailOnError(err, "Failed to create SA tables")

			err = cadb.CreateTablesIfNotExists()
			cmd.FailOnError(err, "Failed to create CA tables")
		}

		// Wire them up
		wfei.RA = &ra
		wfei.SA = sa
		wfei.Stats = stats
		wfei.SubscriberAgreementURL = c.SubscriberAgreementURL

		wfei.IssuerCert, err = cmd.LoadCert(c.CA.IssuerCert)
		cmd.FailOnError(err, fmt.Sprintf("Couldn't read issuer cert [%s]", c.CA.IssuerCert))

		ra.CA = ca
		ra.SA = sa
		ra.VA = &va
		va.RA = &ra
		ca.SA = sa

		// Set up paths
		ra.AuthzBase = c.WFE.BaseURL + wfe.AuthzPath
		wfei.BaseURL = c.WFE.BaseURL
		wfei.HandlePaths()

		auditlogger.Info(app.VersionString())

		fmt.Fprintf(os.Stderr, "Server running, listening on %s...\n", c.WFE.ListenAddress)
		err = http.ListenAndServe(c.WFE.ListenAddress, HandlerTimer(http.DefaultServeMux, stats))
		cmd.FailOnError(err, "Error starting HTTP server")
	}

	app.Run()
}
