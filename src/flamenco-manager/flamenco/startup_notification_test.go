package flamenco

import (
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	log "github.com/sirupsen/logrus"
	httpmock "gopkg.in/jarcoal/httpmock.v1"
)

func TestStartupNotification(t *testing.T) {
	config := GetTestConfig()
	session := MongoSession(&config)

	httpmock.Activate()
	defer httpmock.DeactivateAndReset()

	timeout := make(chan bool, 1)
	go func() {
		time.Sleep(startupNotificationInitialDelay + 250*time.Millisecond)
		timeout <- true
	}()

	httpmock.RegisterResponder(
		"POST",
		"http://localhost:51234/api/flamenco/managers/5852bc5198377351f95d103e/startup",
		func(req *http.Request) (*http.Response, error) {
			// TODO: test contents of request
			log.Info("HTTP POST to Flamenco was performed.")
			defer func() { timeout <- false }()
			return httpmock.NewStringResponse(204, ""), nil
		},
	)

	upstream := ConnectUpstream(&config, session)
	defer upstream.Close()

	notifier := CreateStartupNotifier(&config, upstream, session)
	notifier.Go()
	defer notifier.Close()

	timedout := <-timeout
	assert.False(t, timedout, "HTTP POST to Flamenco not performed")
}
