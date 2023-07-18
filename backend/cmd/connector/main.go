package main

import (
	"log"

	config "openreplay/backend/internal/config/connector"
	"openreplay/backend/internal/connector"
	saver "openreplay/backend/pkg/connector"
	"openreplay/backend/pkg/memory"
	"openreplay/backend/pkg/messages"
	"openreplay/backend/pkg/objectstorage/store"
	"openreplay/backend/pkg/queue"
	"openreplay/backend/pkg/terminator"
)

func main() {
	log.SetFlags(log.LstdFlags | log.LUTC | log.Llongfile)

	cfg := config.New()

	// TODO: specify bucket name for redshift batches
	objStore, err := store.NewStore(&cfg.ObjectsConfig)
	if err != nil {
		log.Fatalf("can't init object storage: %s", err)
	}

	db, err := saver.NewRedshift(cfg)
	if err != nil {
		log.Printf("can't init redshift connection: %s", err)
		return
	}
	defer db.Close()

	// Saves messages to Redshift
	dataSaver := saver.New(cfg, objStore, db)

	// TODO: split into normal and detailed filters
	// Message filter
	//msgFilter := []int{messages.MsgMetadata, messages.MsgIssueEvent, messages.MsgSessionStart, messages.MsgSessionEnd,
	//	messages.MsgUserID, messages.MsgUserAnonymousID, messages.MsgIntegrationEvent, messages.MsgPerformanceTrackAggr,
	//	messages.MsgJSException, messages.MsgResourceTiming, messages.MsgCustomEvent, messages.MsgCustomIssue,
	//	messages.MsgFetch, messages.MsgNetworkRequest, messages.MsgGraphQL, messages.MsgStateAction,
	//	messages.MsgSetInputTarget, messages.MsgSetInputValue, messages.MsgCreateDocument, messages.MsgMouseClick,
	//	messages.MsgSetPageLocation, messages.MsgPageLoadTiming, messages.MsgPageRenderTiming,
	//	messages.MsgInputEvent, messages.MsgPageEvent, messages.MsgMouseThrashing, messages.MsgInputChange,
	//	messages.MsgUnbindNodes}

	// Init consumer
	consumer := queue.NewConsumer(
		cfg.GroupConnector,
		[]string{
			cfg.TopicRawWeb,
			cfg.TopicAnalytics,
		},
		messages.NewMessageIterator(dataSaver.Handle, nil, true),
		false,
		cfg.MessageSizeLimit,
	)

	// Init memory manager
	memoryManager, err := memory.NewManager(cfg.MemoryLimitMB, cfg.MaxMemoryUsage)
	if err != nil {
		log.Printf("can't init memory manager: %s", err)
		return
	}

	// Run service and wait for TERM signal
	service := connector.New(cfg, consumer, dataSaver, memoryManager)
	log.Printf("Db service started\n")
	terminator.Wait(service)
}
