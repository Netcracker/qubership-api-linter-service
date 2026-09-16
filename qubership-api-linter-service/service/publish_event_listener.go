package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sync"

	"github.com/Netcracker/qubership-api-linter-service/client"
	"github.com/Netcracker/qubership-api-linter-service/exception"
	"github.com/Netcracker/qubership-api-linter-service/secctx"
	"github.com/Netcracker/qubership-api-linter-service/utils"
	"github.com/Netcracker/qubership-api-linter-service/view"
	"github.com/buraksezer/olric"
	"github.com/hashicorp/memberlist"
	log "github.com/sirupsen/logrus"
)

type PublishEventListener interface {
	Start()
	listen(message olric.DTopicMessage)
}

func NewPublishEventListener(op client.OlricProvider, validationService ValidationService) PublishEventListener {
	pel := publishEventListenerImpl{
		op:                op,
		validationService: validationService,
		isReadyWg:         sync.WaitGroup{},
	}
	return &pel
}

type publishEventListenerImpl struct {
	op                    client.OlricProvider
	validationService     ValidationService
	versionPublishedTopic *olric.DTopic
	listenerID            uint64
	isReadyWg             sync.WaitGroup
}

func (p *publishEventListenerImpl) Start() {
	log.Infof("PublishEventListener: starting")
	p.isReadyWg.Add(1)
	utils.SafeAsync(func() {
		p.initVersionPublishedDTopic()
	})
	utils.SafeAsync(func() {
		log.Infof("PublishEventListener: watcher goroutine started, waiting for DTopic init to complete")
		p.isReadyWg.Wait()
		log.Infof("PublishEventListener: DTopic init complete, entering node-event watch loop")
		p.watchAndResubscribe()
	})
}

const VersionPublishedTopicName = "version-published"

func (p *publishEventListenerImpl) listen(message olric.DTopicMessage) {
	log.Infof("PublishEventListener.listen: raw message received (publishedAt=%d)", message.PublishedAt)
	str, ok := message.Message.(string)
	if !ok {
		log.Warnf("PublishEventListener.listen: unexpected event %+v, will not be processed", message.Message)
		return
	}
	log.Infof("PublishEventListener.listen: message payload received")

	var notification view.PublishNotification

	err := json.Unmarshal([]byte(str), &notification)
	if err != nil {
		log.Errorf("PublishEventListener.listen: error unmarshalling publish notification: %v", err)
		return
	}

	ctx := secctx.MakeSysadminContext(context.Background())

	version := fmt.Sprintf("%s@%d", notification.Version, notification.Revision)

	taskId, err := p.validationService.ValidateVersion(ctx, notification.PackageId, version, notification.EventId, false)
	if err != nil {
		processed := false
		var customError *exception.CustomError
		if errors.As(err, &customError) {
			if customError.Code == exception.DuplicateEvent {
				log.Infof("PublishEventListener.listen: event with id=%s is already processed", notification.EventId)
				processed = true
			}
			if customError.Code == exception.LintNotSupported {
				log.Infof("PublishEventListener.listen: event with id=%s is for not supported package kind, skipping it", notification.EventId)
				processed = true
			}
		}
		if !processed {
			log.Errorf("PublishEventListener.listen: error in version %+v validation: %v", "", err)
		}
		return
	}
	log.Infof("Lint task with id=%s is created for event %+v", taskId, notification)
}

func (p *publishEventListenerImpl) initVersionPublishedDTopic() {
	log.Infof("PublishEventListener: creating DTopic %s", VersionPublishedTopicName)
	var err error
	p.versionPublishedTopic, err = p.op.Get().NewDTopic(VersionPublishedTopicName, 10000, olric.UnorderedDelivery)
	if err != nil {
		log.Errorf("Failed to create DTopic %s: %s", VersionPublishedTopicName, err.Error())
		p.isReadyWg.Done()
		return
	}
	log.Infof("PublishEventListener: DTopic %s created, adding listener", VersionPublishedTopicName)

	p.listenerID, err = p.versionPublishedTopic.AddListener(p.listen)
	if err != nil {
		log.Errorf("Failed to add listener to DTopic %s: %s", VersionPublishedTopicName, err.Error())
	} else {
		log.Infof("PublishEventListener: listener registered on DTopic %s (listenerID=%d)", VersionPublishedTopicName, p.listenerID)
	}

	p.isReadyWg.Done()
}

func (p *publishEventListenerImpl) watchAndResubscribe() {
	for event := range p.op.NodeEvents() {
		switch event.Event {
		case memberlist.NodeLeave:
			log.Warnf("PublishEventListener: peer %s left cluster, topic %s delivery is interrupted until it rejoins", event.Node.Name, VersionPublishedTopicName)
		case memberlist.NodeJoin:
			log.Infof("PublishEventListener: peer %s joined cluster, re-subscribing to topic %s", event.Node.Name, VersionPublishedTopicName)
			p.resubscribe()
		case memberlist.NodeUpdate:
			log.Infof("PublishEventListener: peer %s metadata updated", event.Node.Name)
		}
	}
	log.Infof("PublishEventListener: node-event channel closed, watch loop exiting")
}

func (p *publishEventListenerImpl) resubscribe() {
	log.Infof("PublishEventListener: resubscribe started (currentListenerID=%d)", p.listenerID)
	if p.versionPublishedTopic != nil && p.listenerID != 0 {
		log.Infof("PublishEventListener: removing stale listener (listenerID=%d)", p.listenerID)
		if err := p.versionPublishedTopic.RemoveListener(p.listenerID); err != nil {
			log.Warnf("PublishEventListener: failed to remove stale listener from topic %s: %v", VersionPublishedTopicName, err)
		} else {
			log.Infof("PublishEventListener: stale listener removed (listenerID=%d)", p.listenerID)
		}
	}

	log.Infof("PublishEventListener: recreating DTopic %s", VersionPublishedTopicName)
	var err error
	p.versionPublishedTopic, err = p.op.Get().NewDTopic(VersionPublishedTopicName, 10000, olric.UnorderedDelivery)
	if err != nil {
		log.Errorf("PublishEventListener: failed to recreate DTopic %s on peer join: %v", VersionPublishedTopicName, err)
		p.listenerID = 0
		return
	}
	log.Infof("PublishEventListener: DTopic %s recreated, re-adding listener", VersionPublishedTopicName)

	p.listenerID, err = p.versionPublishedTopic.AddListener(p.listen)
	if err != nil {
		log.Errorf("PublishEventListener: failed to re-add listener to DTopic %s on peer join: %v", VersionPublishedTopicName, err)
	} else {
		log.Infof("PublishEventListener: listener re-registered on DTopic %s (listenerID=%d)", VersionPublishedTopicName, p.listenerID)
	}
}
