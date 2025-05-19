package main

import (
	"context"
	"database/sql"
	"flag"
	"github.com/grannnsacker/job-finder-back/internal/api"
	"github.com/grannnsacker/job-finder-back/internal/config"
	db "github.com/grannnsacker/job-finder-back/internal/db/sqlc"
	"github.com/grannnsacker/job-finder-back/internal/esearch"
	zerolog "github.com/rs/zerolog/log"
	rabbitmq "github.com/streadway/amqp"
	"log"
)

func main() {
	// === config, env file ===
	cfg, err := config.LoadConfig(".")
	if err != nil {
		zerolog.Fatal().Err(err).Msg("cannot load env file")
	}

	// === database ===
	conn, err := sql.Open(cfg.DBDriver, cfg.DBSource)
	if err != nil {
		zerolog.Fatal().Err(err).Msg("cannot connect to the db")
	}

	store := db.NewStore(conn)

	// === loading test data ===
	loadDataFlag := flag.Bool("load_test_data", false, "If set, the application will load test data into db")
	flag.Parse()

	if *loadDataFlag != false {
		// load the test data into the db
		store.LoadTestData(context.Background())
	}

	// === Elasticsearch ===
	ctx := context.Background()
	ctx, err = esearch.LoadJobsFromDB(ctx, store)
	if err != nil {
		zerolog.Fatal().Err(err).Msg("cannot load jobs from db")
	}
	newClient, err := esearch.ConnectWithElasticsearch(cfg.ElasticSearchAddress)
	if err != nil {
		zerolog.Fatal().Err(err).Msg("cannot connect to the elasticsearch")
	}

	client := esearch.NewClient(newClient)
	err = client.IndexJobsAsDocuments(ctx)
	if err != nil {
		zerolog.Fatal().Err(err).Msg("cannot index jobs as documents")
	}

	connRabit, err := rabbitmq.Dial("amqp://devuser:admin@rabbitmq:5672/")
	if err != nil {
		log.Fatalf("Failed to connect: %v", err)
	}
	defer connRabit.Close()

	// Создание канала
	ch, err := connRabit.Channel()
	if err != nil {
		log.Fatalf("Failed to declare channel: %v", err)
	}
	defer ch.Close()

	q, err := ch.QueueDeclare(
		"telegram_notifications",
		true,  // durable
		false, // auto-delete
		false, // exclusive
		false, // no-wait
		nil,
	)
	if err != nil {
		log.Fatal("Failed to declare queue:", err)
	}

	// === HTTP server ===
	runHTTPServer(cfg, store, client, ch, q)
}

func runHTTPServer(cfg config.Config, store db.Store, client esearch.ESearchClient, ch *rabbitmq.Channel, q rabbitmq.Queue) {
	server, err := api.NewServer(cfg, store, client, ch, q)
	if err != nil {
		zerolog.Fatal().Err(err).Msg("cannot create server")
	}

	// @contact.name aalug
	// @contact.url https://github.com/aalug
	// @contact.email a.a.gulczynski@gmail.com
	// @securityDefinitions.apikey ApiKeyAuth
	// @in header
	// @name Authorization
	// @description Use 'bearer {token}' without quotes.
	err = server.Start(cfg.ServerAddress)
	if err != nil {
		zerolog.Fatal().Err(err).Msg("cannot start the server")
	}
}
