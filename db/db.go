package db

import (
	"SalesforceGit/cdcSubscribe/common"
	"context"
	"fmt"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"log"
)

var ctx = context.TODO()
var client *mongo.Client
var db *mongo.Database

func InitDB() error {
	clientOptions := options.Client().ApplyURI("mongodb://root:example@localhost:27017/")
	var err error
	client, err = mongo.Connect(ctx, clientOptions)

	if err != nil {
		return err
	}

	err = client.Ping(ctx, nil)

	if err != nil {
		return err
	} else {
		log.Println("Pinged!")
	}

	db = client.Database("SalesforceCDC")
	return nil
}

func GetListOfCollections() ([]string, error) {
	return db.ListCollectionNames(ctx, bson.D{})
}

func ConvertBSONDtoMap(objects []bson.D) []map[string]interface{} {
	var result []map[string]interface{}

	for _, obj := range objects {
		m := make(map[string]interface{})
		for _, elem := range obj {
			m[elem.Key] = elem.Value
		}
		result = append(result, m)
	}

	return result
}
func GetEventsById(modelName string, id string) ([]map[string][]bson.D, error) {
	collection := db.Collection("/data/" + modelName + "ChangeEvent")
	var err error

	// Define a projection to exclude the "Events" field
	filter := bson.D{{"recordIds", id}}
	projection := bson.D{{"Events", 1}}

	// Perform the find operation with projection
	cursor, err := collection.Find(context.Background(), filter, options.Find().SetProjection(projection))
	if err != nil {
		panic(err)
	}
	defer cursor.Close(context.Background())

	var objects []bson.D
	for cursor.Next(context.Background()) {
		var doc bson.D
		err := cursor.Decode(&doc)
		if err != nil {
			panic(err)
		}
		objects = append(objects, doc)
	}

	events := ConvertBSONDtoMap(objects)
	var bsonArray []bson.D
	for _, value := range events {
		if primitiveArray, ok := value["Events"].(primitive.A); ok {
			bsonArray = ConvertPrimitiveAToBSOND(primitiveArray)
		} else {
			fmt.Println("value[Events] is not a primitive.A")
		}
	}
	filteredEvents := ConvertBsonArrayToMap(bsonArray)
	return filteredEvents, nil
}

func ConvertPrimitiveAToBSOND(primitiveArray primitive.A) []bson.D {
	var bsonArray []bson.D
	for _, item := range primitiveArray {
		// Type assert the item to bson.D
		if bsonDoc, ok := item.(bson.D); ok {
			bsonArray = append(bsonArray, bsonDoc)
		} else {
			fmt.Println("Item is not a bson.D")
		}
	}
	return bsonArray
}

func GetChangedObjects(modelName string) ([]bson.D, error) {
	collection := db.Collection("/data/" + modelName + "ChangeEvent")
	var err error
	var objects []bson.D
	// Define a projection to exclude the "Events" field
	projection := bson.D{{"Events", 0}}
	//var objectsMap []map[string]interface{}
	// Perform the find operation with projection
	cursor, err := collection.Find(context.Background(), bson.D{}, options.Find().SetProjection(projection))
	if err != nil {
		return objects, err
	}
	defer cursor.Close(context.Background())

	for cursor.Next(context.Background()) {
		var doc bson.D
		err := cursor.Decode(&doc)
		if err != nil {
			panic(err)
		}

		objects = append(objects, doc)
	}

	//objectsMap = ConvertBSONDtoMap(objects)

	return objects, nil
}

func GetSubscribeValues() (common.IntegrationUserValues, error) {
	collection := db.Collection("integrationUserValues")
	var data common.IntegrationUserValues

	cursor, err := collection.Find(context.TODO(), bson.D{})
	if err != nil {
		return data, err
	}
	defer cursor.Close(context.TODO())

	// Check if cursor has any documents
	if !cursor.Next(context.TODO()) {
		return data, fmt.Errorf("no documents found")
	}

	// Decode the first document
	if err := cursor.Decode(&data); err != nil {
		return data, err
	}

	return data, nil
}

func createIntegrationUser(subscribeValues common.IntegrationUserValues) error {
	collection := db.Collection("integrationUserValues")
	subscribeValues.Topics = []common.Topic{}
	_, err := collection.InsertOne(context.TODO(), subscribeValues)
	if err != nil {
		return err
	}

	return nil
}

func UpdateSubscribeValues(updatedSubscribeValues common.IntegrationUserValues) error {
	collection := db.Collection("integrationUserValues")
	previousData, err := GetSubscribeValues()
	if previousData.Username != "" || previousData.SalesforceKey != "" || len(previousData.Topics) != 0 {
		err = UpdateConsumerKeyAndUsername(previousData.Username, updatedSubscribeValues.SalesforceKey, updatedSubscribeValues.Username)
	} else {
		err = createIntegrationUser(updatedSubscribeValues)
	}

	err = UpdateConsumerKeyAndUsername(updatedSubscribeValues.Username, updatedSubscribeValues.SalesforceKey, updatedSubscribeValues.Username)

	if err != nil {
		return err
	}

	if err != nil && err.Error() == "no documents found" {
		_, err = collection.InsertOne(ctx, updatedSubscribeValues)
		if err != nil {
			return err
		}
		return nil
	} else if err != nil {
		return err
	}

	// delete empty topics
	var filteredTopics []common.Topic
	for _, el := range updatedSubscribeValues.Topics {
		if el.ChannelName != "" {
			filteredTopics = append(filteredTopics, el)
		}
	}

	// update old topics and delete deleted
	for _, oldTopic := range previousData.Topics {
		found := false
		for _, newTopic := range filteredTopics {
			if oldTopic.ChannelName == newTopic.ChannelName {
				oldTopic.CurrentlySubscribed = newTopic.CurrentlySubscribed
				err = UpdateTopic(oldTopic)
				found = true
			}
		}
		if !found {
			err = DeleteTopic(oldTopic, updatedSubscribeValues.Username)
			if err != nil {
				return err
			}
		}
	}

	// insert new topic
	for _, newTopic := range filteredTopics {
		found := false
		for _, oldTopic := range previousData.Topics {
			if oldTopic.ChannelName == newTopic.ChannelName {
				found = true
			}
		}
		if !found {
			err := InsertTopic(newTopic, updatedSubscribeValues.Username)
			if err != nil {
				return err
			}
		}
	}

	return nil
}

func UpdateTopic(topic common.Topic) error {
	collection := db.Collection("integrationUserValues")
	filter := bson.M{"Topics.channelname": topic.ChannelName}

	update := bson.D{{"$set", bson.D{{"Topics.$", topic}}}}
	updateResult, err := collection.UpdateOne(ctx, filter, update)
	if err != nil {
		return err
	}

	// Check if any documents were modified
	if updateResult.MatchedCount == 0 {
		return fmt.Errorf("no document found with ChannelName: %s", topic.ChannelName)
	}

	log.Printf("Successfully updated topic with ChannelName: %s", topic.ChannelName)
	return nil
}

func InsertTopic(topic common.Topic, Username string) error {
	collection := db.Collection("integrationUserValues")

	filter := bson.M{"Username": Username}

	update := bson.D{{"$push", bson.D{{"Topics", topic}}}}

	updateResult, err := collection.UpdateOne(ctx, filter, update)
	if err != nil {
		return err
	}

	if updateResult.MatchedCount == 0 {
		return fmt.Errorf("no document found with Username: %s", Username)
	}

	log.Printf("Successfully inserted topic into document with Username: %s", Username)
	return nil
}

func DeleteTopic(topic common.Topic, Username string) error {
	collection := db.Collection("integrationUserValues")

	filter := bson.M{"Username": Username}
	update := bson.D{{"$pull", bson.D{{"Topics", bson.M{"channelname": topic.ChannelName}}}}}

	updateResult, err := collection.UpdateOne(ctx, filter, update)
	if err != nil {
		return err
	}

	if updateResult.MatchedCount == 0 {
		return fmt.Errorf("no document found with Username: %s", Username)
	}

	log.Printf("Successfully deleted topic from document with Username: %s", Username)
	return nil
}

func UpdateConsumerKeyAndUsername(username string, newConsumerKey string, newUsername string) error {
	collection := db.Collection("integrationUserValues")

	filter := bson.M{"Username": username}

	update := bson.D{
		{"$set", bson.D{
			{"Username", newUsername},
			{"SalesforceKey", newConsumerKey},
		}},
	}

	updateResult, err := collection.UpdateOne(ctx, filter, update)
	if err != nil {
		return err
	}

	if updateResult.MatchedCount == 0 {
		return fmt.Errorf("no document found with Username: %s", username)
	}

	log.Printf("Successfully updated ConsumerKey and Username in document with Username: %s", username)
	return nil
}

func SaveEventToDB(topic string, body map[string]interface{}) error {
	collection := db.Collection(topic)
	changeEventHeader := setupChangeEventHeader(topic, body["ChangeEventHeader"])
	lastModifiedDate := body["LastModifiedDate"]
	commitUser := changeEventHeader["commitUser"]
	recordIds := changeEventHeader["recordIds"]
	body["ChangeEventHeader"] = changeEventHeader
	update := bson.M{
		"$push": bson.M{
			"Events": body,
		},
		"$set": bson.M{
			"lastModifiedDate": lastModifiedDate,
			"commitUser":       commitUser,
		},
	}
	_, err := collection.UpdateOne(context.TODO(), bson.M{"recordIds": recordIds}, update, options.Update().SetUpsert(true))

	//_, err := collection.InsertOne(context.TODO(), body)
	if err != nil {
		log.Fatal(err)
	}
	return nil
}

func UpdateSchema(topicName string, schema string) {
	// add here finding by topic

	common.TestSchemas[topicName] = ParseSchema(topicName, schema)

	common.Schema[topicName] = schema

	//common.FieldsNames = GetFieldsNames(schema)

}
