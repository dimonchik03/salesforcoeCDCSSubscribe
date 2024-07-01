package subscribe

import (
	"SalesforceGit/db"
	"errors"
	"fmt"
	"log"

	"SalesforceGit/cdcSubscribe/common"
	"SalesforceGit/cdcSubscribe/grpcclient"
	"SalesforceGit/cdcSubscribe/proto"
)

var Clients []*grpcclient.PubSubClient

func beginSubscribe(newTopic common.Topic) error {
	if common.ReplayPreset == proto.ReplayPreset_CUSTOM && common.ReplayId == nil {
		log.Fatalf("the replayId variable must be populated when the replayPreset variable is set to CUSTOM")
	} else if common.ReplayPreset != proto.ReplayPreset_CUSTOM && common.ReplayId != nil {
		log.Fatalf("the replayId variable must not be populated when the replayPreset variable is set to EARLIEST or LATEST")
	}

	log.Printf("Creating gRPC client...")
	client, err := grpcclient.NewGRPCClient()
	if err != nil {
		//log.Fatalf("could not create gRPC client: %v", err)
		return err
	}
	(*client).Topic = newTopic

	defer client.Close()

	log.Printf("Populating auth token...")
	err = client.Authenticate()
	if err != nil {
		client.Close()
		errorString := fmt.Sprintf("could not authenticate: %v", err.Error())

		return errors.New(errorString)
	}

	log.Printf("Populating user info...")
	err = client.FetchUserInfo()
	if err != nil {
		client.Close()
		errorString := fmt.Sprintf("could not fetch user info: %v", err.Error())

		return errors.New(errorString)
	}

	log.Printf("Making GetTopic request...")
	topic, err := client.GetTopic()
	if err != nil {
		client.Close()
		//log.Fatalf("could not fetch topic: %v", err)
		errorString := fmt.Sprintf("could not fetch topic: %v", err.Error())

		return errors.New(errorString)
	}

	if !topic.GetCanSubscribe() {
		client.Close()
		//log.Fatalf("this user is not allowed to subscribe to the following topic: %s", client.Topic.ChannelName)
		errorString := fmt.Sprintf("this user is not allowed to subscribe to the following topic: %s", client.Topic.ChannelName)

		return errors.New(errorString)
	}
	client.Topic.Error = ""
	Clients = append(Clients, client)

	err = db.UpdateTopic(client.Topic)

	if err != nil {
		errorString := fmt.Sprintf("could not update topic in db, %v", err.Error())
		return errors.New(errorString)
	}
	curReplayId := common.ReplayId
	for {
		log.Printf("Subscribing to topic...")

		// use the user-provided ReplayPreset by default, but if the curReplayId variable has a non-nil value then assume that we want to
		// consume from a custom offset. The curReplayId will have a non-nil value if the user explicitly set the ReplayId or if a previous
		// subscription attempt successfully processed at least one event before crashing
		replayPreset := common.ReplayPreset
		if curReplayId != nil {
			replayPreset = proto.ReplayPreset_CUSTOM
		}

		// In the happy path the Subscribe method should never return, it will just process events indefinitely. In the unhappy path
		// (i.e., an error occurred) the Subscribe method will return both the most recently processed ReplayId as well as the error message.
		// The error message will be logged for the user to see and then we will attempt to re-subscribe with the ReplayId on the next iteration
		// of this for loop
		curReplayId, err = client.Subscribe(replayPreset, curReplayId)
		if err != nil {
			log.Printf("error occurred while subscribing to topic %v: %v", client.Topic.ChannelName, err)
			errorString := fmt.Sprintf("error occurred while subscribing to topic %v: %v", client.Topic.ChannelName, err)
			//(*client).Topic.Error = errorString
			//(*client).Topic.CurrentlySubscribed = false
			return errors.New(errorString)
		}
	}
}

func goRoutineToSubscribe(topic common.Topic) {

}

func UpdateSubscribe() error {
	//var subscribedTopics []common.Topic
	subscribeValues, err := db.GetSubscribeValues()
	// here we need to check if we are already subscribed or were subscribed to close it
	if err != nil {
		return err
	}
	//amountOfDeletedItems := 0

	for i, topic := range subscribeValues.Topics {
		go func(i int, topic common.Topic) {
			// check whether already subscribed
			if topic.CurrentlySubscribed && !isNowSubscribed(topic) {
				subscribeValues.Topics[i].Error = beginSubscribe(topic).Error()
				subscribeValues.Topics[i].CurrentlySubscribed = false
				_ = db.UpdateTopic(subscribeValues.Topics[i])
			} else if !topic.CurrentlySubscribed && isNowSubscribed(topic) {
				newClients := make([]*grpcclient.PubSubClient, 0)
				for _, client := range Clients {
					if client.Topic.ChannelName != topic.ChannelName {
						newClients = append(newClients, client)
					} else {
						client.Close()
					}
				}
				Clients = newClients
			}
		}(i, topic)
	}

	stopDeletedSubscribes(subscribeValues.Topics)
	// update values

	fmt.Println("subscribeValues in subscribe after update  ")
	fmt.Println(subscribeValues)
	//err = db.UpdateSubscribeValues(subscribeValues)

	return nil
}

func isNowSubscribed(topic common.Topic) bool {
	// see whether client with such a topic exists
	for _, client := range Clients {
		if client.Topic.ChannelName == topic.ChannelName {
			return true
		}
	}
	return false
}

func stopDeletedSubscribes(subscribeTopics []common.Topic) {
	amountOfDeletedItems := 0
	for i, client := range Clients {
		found := false
		for _, topic := range subscribeTopics {
			if client.Topic.ChannelName == topic.ChannelName {
				found = true
				break
			}
		}
		if !found {
			client.Close()
			client.Topic.CurrentlySubscribed = false
			//_ = db.UpdateTopic(client.Topic)
			Clients = append(Clients[:i-amountOfDeletedItems], Clients[i-amountOfDeletedItems+1:]...)
			amountOfDeletedItems++
		}
	}
}
