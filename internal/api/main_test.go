package api

import (
	"github.com/gin-gonic/gin"
	"github.com/grannnsacker/job-finder-back/internal/config"
	"github.com/grannnsacker/job-finder-back/internal/db/sqlc"
	"github.com/grannnsacker/job-finder-back/internal/esearch"
	"github.com/grannnsacker/job-finder-back/pkg/utils"
	rabbitmq "github.com/streadway/amqp"
	"github.com/stretchr/testify/require"
	"os"
	"testing"
	"time"
)

func newTestServer(t *testing.T, store db.Store, client esearch.ESearchClient) *Server {
	cfg := config.Config{
		TokenSymmetricKey:   utils.RandomString(32),
		AccessTokenDuration: time.Minute,
	}
	q := rabbitmq.Queue{}
	server, err := NewServer(cfg, store, client, nil, q)
	require.NoError(t, err)

	if client != nil {
		server.esDetails.lastDocumentIndex = 1
	}

	return server
}

func TestMain(m *testing.M) {
	gin.SetMode(gin.TestMode)

	os.Exit(m.Run())
}
