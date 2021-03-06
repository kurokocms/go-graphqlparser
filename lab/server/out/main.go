package main

import (
	"encoding/json"
	"fmt"
	"log"

	"github.com/bucketd/go-graphqlparser/lab/server"
)

func main() {
	response := server.Response{
		Data: server.ResponseValue{
			Kind: server.ResponseValueKindObject,
			ObjectValue: []server.ResponseObjectField{
				{Name: "f1", Value: server.ResponseValue{
					Kind: server.ResponseValueKindObject,
					ObjectValue: []server.ResponseObjectField{
						{Name: "episodeID", Value: server.ResponseValue{
							Kind:     server.ResponseValueKindInt,
							IntValue: 4,
						}},
					},
				}},
				{Name: "f2", Value: server.ResponseValue{
					Kind: server.ResponseValueKindObject,
					ObjectValue: []server.ResponseObjectField{
						{Name: "title", Value: server.ResponseValue{
							Kind:        server.ResponseValueKindString,
							StringValue: "Attack of the Clones",
						}},
					},
				}},
				{Name: "f3", Value: server.ResponseValue{
					Kind: server.ResponseValueKindArray,
					ArrayValue: []server.ResponseValue{
						{Kind: server.ResponseValueKindInt, IntValue: 123},
						{Kind: server.ResponseValueKindFloat, FloatValue: 123.456},
						{Kind: server.ResponseValueKindNull},
						{Kind: server.ResponseValueKindBoolean, BooleanValue: true},
						{Kind: server.ResponseValueKindBoolean, BooleanValue: false},
						{Kind: server.ResponseValueKindString, StringValue: "test"},
					},
				}},
			},
		},
	}

	res, err := response.MarshalGraphQL()
	if err != nil {
		log.Fatal(err)
	}

	fmt.Println(string(res))

	mapRes := make(map[string]interface{})
	mapRes["data"] = map[string]interface{}{
		"f1": map[string]interface{}{
			"episodeID": 4,
		},
		"f2": map[string]interface{}{
			"title": "Attack of the Clones",
		},
	}

	mapBS, err := json.Marshal(mapRes)
	if err != nil {
		log.Fatal(err)
	}

	fmt.Println(string(mapBS))
}
