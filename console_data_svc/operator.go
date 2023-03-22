//
//  MIT License
//
//  (C) Copyright 2023 Hewlett Packard Enterprise Development LP
//
//  Permission is hereby granted, free of charge, to any person obtaining a
//  copy of this software and associated documentation files (the "Software"),
//  to deal in the Software without restriction, including without limitation
//  the rights to use, copy, modify, merge, publish, distribute, sublicense,
//  and/or sell copies of the Software, and to permit persons to whom the
//  Software is furnished to do so, subject to the following conditions:
//
//  The above copyright notice and this permission notice shall be included
//  in all copies or substantial portions of the Software.
//
//  THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
//  IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
//  FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL
//  THE AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR
//  OTHER LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE,
//  ARISING FROM, OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR
//  OTHER DEALINGS IN THE SOFTWARE.
//

package main

import (
	"encoding/json"
	"fmt"
	"log"
)

var operatorAddress string = "http://cray-console-operator/console-operator/v1"

type GetNodeReplicasResponse struct {
	Replicas int `json:"replicas"`
}

// Calls console operator /replicas for current node replica count
func getConsoleNodeReplicas() (replicas int, err error) {
	url := fmt.Sprintf("%s/replicas", operatorAddress)
	rd, _, err := getURL(url, nil)
	if err != nil {
		log.Printf("Error: Failed to GET to %s\n", url)
		return -1, err
	}

	var replicasResp GetNodeReplicasResponse
	json_err := json.Unmarshal(rd, &replicasResp)
	if json_err != nil {
		log.Printf("Error: There was an error while decoding the JSON data: %s\n", json_err)
		return -1, err
	}

	return replicasResp.Replicas, nil
}
