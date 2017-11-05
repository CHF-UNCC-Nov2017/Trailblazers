/*
Licensed to the Apache Software Foundation (ASF) under one
or more contributor license agreements.  See the NOTICE file
distributed with this work for additional information
regarding copyright ownership.  The ASF licenses this file
to you under the Apache License, Version 2.0 (the
"License"); you may not use this file except in compliance
with the License.  You may obtain a copy of the License at

  http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing,
software distributed under the License is distributed on an
"AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
KIND, either express or implied.  See the License for the
specific language governing permissions and limitations
under the License.
*/

// ====CHAINCODE EXECUTION SAMPLES (BCS REST API) ==================

//#TEST transaction / Init ledger


curl -H "Content-type:application/json" -X POST http://localhost:3100/bcsgw/rest/v1/transaction/invocation -d '{"channel":"securechannel","chaincode":"customerkyc10","method":"initCustomer","args":["BankId", "BankName", "customerName", "SSN", "passportNo", "Address", "Phone", "email","AccountType"],"chaincodeVer":"v1"}'

//# TEST query / Populated database
//# TEST query / Get History

curl -H "Content-type:application/json" -X POST http://localhost:3100/bcsgw/rest/v1/transaction/query -d '{"channel":"channel1","chaincode":"vehiclenet","method":"getHistoryForRecord","args":["ser1234"],"chaincodeVer":"v1"}'
curl -H "Content-type:application/json" -X POST http://localhost:3100/bcsgw/rest/v1/transaction/query -d '{"channel":"channel1","chaincode":"vehiclenet","method":"getHistoryForRecord","args":["mer1000001"],"chaincodeVer":"v1"}'

//CRYPTO
//#Sign
//go run cryptoHOL.go -s welcome

//#Verify
//go run cryptoHOL.go -v welcome 23465785510810132448457841429882907809251724155505686786147550387897 //10848776947772665661803987914449872333300709981875993855742805426849


// Index for chaincodeid, docType, owner, size (descending order).
// Note that docType, owner and size fields must be prefixed with the "data" wrapper
// chaincodeid must be added for all queries
//
// Definition for use with Fauxton interface
// {"index":{"fields":[{"data.size":"desc"},{"chaincodeid":"desc"},{"data.docType":"desc"},{"data.owner":"desc"}]},"ddoc":"indexSizeSortDoc", "name":"indexSizeSortDesc","type":"json"}
//
// example curl definition for use with command line
// curl -i -X POST -H "Content-Type: application/json" -d "{\"index\":{\"fields\":[{\"data.size\":\"desc\"},{\"chaincodeid\":\"desc\"},{\"data.docType\":\"desc\"},{\"data.owner\":\"desc\"}]},\"ddoc\":\"indexSizeSortDoc\", \"name\":\"indexSizeSortDesc\",\"type\":\"json\"}" http://hostname:port/channelNameGoesHere/_index

// Rich Query with index design doc and index name specified (Only supported if CouchDB is used as state database):
//   peer chaincode query -C channelNameGoesHere -n vehicleParts -c '{"Args":["queryVehiclePart","{\"selector\":{\"docType\":\"vehiclePart\",\"owner\":\"mercedes\"}, \"use_index\":[\"_design/indexOwnerDoc\", \"indexOwner\"]}"]}'

// Rich Query with index design doc specified only (Only supported if CouchDB is used as state database):
//   peer chaincode query -C channelNameGoesHere -n vehicleParts -c '{"Args":["queryVehiclePart","{\"selector\":{\"docType\":{\"$eq\":\"vehiclePart\"},\"owner\":{\"$eq\":\"mercedes\"},\"assemblyDate\":{\"$gt\":1502688979}},\"fields\":[\"docType\",\"owner\",\"assemblyDate\"],\"sort\":[{\"assemblyDate\":\"desc\"}],\"use_index\":\"_design/indexSizeSortDoc\"}"]}'

package main

import (
	"bytes"
	"crypto/ecdsa"
	"crypto/x509"
	"encoding/json"
	"fmt"
	"math/big"
	"strconv"
	"strings"
	"time"

	"github.com/hyperledger/fabric/core/chaincode/shim"
	pb "github.com/hyperledger/fabric/protos/peer"
)

// AutoTraceChaincode example simple Chaincode implementation
type AutoTraceChaincode struct {
}

// @MODIFY_HERE add recall fields to vehicle JSON object
type customer struct {
	bankId        		string `json:"bankId"`       //docType is used to distinguish the various types of objects in state database
	bankName      string `json:"bankName"` //the fieldtags are needed to keep case from bouncing around
	customerName       string `json:"customerName"`
	SSN              string `json:"ssn"`
	Passport       string    `json:"passport"`
	Address  	string `json:"address"`
	Phone              string `json:"phone"`
	EmailId       string    `json:"emailId"`
	AccountType  	string `json:"accountType"`
	AccountNo              string `json:"accountNo"`
	TransactionHistory  int `json:"transactionHistory"` 
	CreditHistory  int `json:"creditHistory"`
	AccountStatus string `json:"accountStatus"`	
}

// ===================================================================================
// Main
// ===================================================================================
func main() {
	err := shim.Start(new(AutoTraceChaincode))
	if err != nil {
		fmt.Printf("Error starting Parts Trace chaincode: %s", err)
	}
}

// Init initializes chaincode
// ===========================
func (t *AutoTraceChaincode) Init(stub shim.ChaincodeStubInterface) pb.Response {
	fmt.Println("Building ledger initial state")
	// ==== Only enable this to be executed once by checking for a pre-installed serial number ====
/*	vehiclePartAsBytes, err := stub.GetState("abg1234")
	if vehiclePartAsBytes != nil {
		fmt.Println("ledger state previously set")
		return shim.Success(nil)
	}

	message, err := t.initLedgerA(stub)
	if err != nil {
		return shim.Error(err.Error())
	} else if message != "" {
		return shim.Error("Failed to run initLedgerA:" + message)
	}
*/	
	fmt.Println("ledger initial state set")
	return shim.Success(nil)
}

// Invoke - Our entry point for Invocations
// ========================================
func (t *AutoTraceChaincode) Invoke(stub shim.ChaincodeStubInterface) pb.Response {
	function, args := stub.GetFunctionAndParameters()
	fmt.Println("invoke is running " + function)

	// Handle different functions
	if function == "getCustomerRecord" { //get history of values for a record
		return t.getCustomerRecord(stub, args)
	} else if function == "initCustomer" { //create a new vehicle
		return t.initCustomer(stub, args)
	} 
	fmt.Println("invoke did not find func: " + function) //error
	return shim.Error("Received unknown function invocation")
}

// ============================================================
// initVehicle - create a new new customer , store into chaincode state
// ============================================================
func (t *AutoTraceChaincode) initCustomer(stub shim.ChaincodeStubInterface, args []string) pb.Response {
	var err error

	// data model with recall fields
	//   0       		1      		2     		3			   4		5	       6			7
	// "BankId", "BankName", "customerName", "SSN", "passport", "address", "phone", "emailid" ,"Accounttype"



	// @MODIFY_HERE extend to expect 8 arguements, up from 6
	if len(args) != 9 {
		return shim.Error("Incorrect number of arguments. Expecting 7")
	}

	// ==== Input sanitation ====
	fmt.Println("- start init customer")
	if len(args[0]) <= 0 {
		return shim.Error("1st argument must be a non-empty string")
	}
	if len(args[1]) <= 0 {
		return shim.Error("2nd argument must be a non-empty string")
	}
	if len(args[2]) <= 0 {
		return shim.Error("3rd argument must be a non-empty string")
	}
	if len(args[4]) <= 0 {
		return shim.Error("5th argument must be a non-empty string")
	}
	if len(args[5]) <= 0 {
		return shim.Error("6th argument must be a non-empty string")
	}

	bankId := args[0]
	bankName := strings.ToLower(args[1])
	customerName := strings.ToLower(args[2])
	SSN := strings.ToLower(args[3])
	Passport := strings.ToLower(args[4])
	Address := strings.ToLower(args[5])
	Phone := strings.ToLower(args[6])
	EmailId := strings.ToLower(args[7])
	AccountType := strings.ToLower(args[8])
	AccountNo := "acnt_10";
	TransactionHistory := 0
	CreditHistory := 0
	AccountStatus := "Active"

	// @MODIFY_HERE parts recall fields
	// ==== Create vehicle object and marshal to JSON ====
//	objectType := "customer"
	//vehicle := &vehicle{objectType, chassisNumber, manufacturer, model, assemblyDate, airbagSerialNumber, owner}
	customer := &customer{bankId, bankName, customerName, SSN, Passport, Address, Phone, EmailId, AccountType, AccountNo,TransactionHistory, CreditHistory, AccountStatus}
	customerJSONasBytes, err := json.Marshal(customer)
	if err != nil {
		return shim.Error(err.Error())
		//return shim.Error(err.Error())
	}

	// === Save customer to state ===
	err = stub.PutState(SSN, customerJSONasBytes)
	if err != nil {
		return shim.Error(err.Error())
	}
	
	// ==== Customer saved. Return success ====
	fmt.Println("- end init customer")
	return shim.Success(nil)
}

// ===========================================================================================
// getHistoryForRecord returns the histotical state transitions for a given key of a record
// ===========================================================================================
func (t *AutoTraceChaincode) getCustomerRecord(stub shim.ChaincodeStubInterface, args []string) pb.Response {

	if len(args) < 1 {
		return shim.Error("Incorrect number of arguments. Expecting 1")
	}

	recordKey := args[0]

	fmt.Printf("- start getHistoryForRecord: %s\n", recordKey)

	resultsIterator, err := stub.GetHistoryForKey(recordKey)
	if err != nil {
		return shim.Error(err.Error())
	}
	defer resultsIterator.Close()

	// buffer is a JSON array containing historic values for the key/value pair
	var buffer bytes.Buffer
	buffer.WriteString("[")

	bArrayMemberAlreadyWritten := false
	for resultsIterator.HasNext() {
		response, err := resultsIterator.Next()
		if err != nil {
			return shim.Error(err.Error())
		}
		// Add a comma before array members, suppress it for the first array member
		if bArrayMemberAlreadyWritten == true {
			buffer.WriteString(",")
		}
		buffer.WriteString("{\"TxId\":")
		buffer.WriteString("\"")
		buffer.WriteString(response.TxId)
		buffer.WriteString("\"")

		buffer.WriteString(", \"Value\":")
		// if it was a delete operation on given key, then we need to set the
		//corresponding value null. Else, we will write the response.Value
		//as-is (as the Value itself a JSON vehiclePart)
		if response.IsDelete {
			buffer.WriteString("null")
		} else {
			buffer.WriteString(string(response.Value))
		}

		buffer.WriteString(", \"Timestamp\":")
		buffer.WriteString("\"")
		buffer.WriteString(time.Unix(response.Timestamp.Seconds, int64(response.Timestamp.Nanos)).String())
		buffer.WriteString("\"")

		buffer.WriteString(", \"IsDelete\":")
		buffer.WriteString("\"")
		buffer.WriteString(strconv.FormatBool(response.IsDelete))
		buffer.WriteString("\"")

		buffer.WriteString("}")
		bArrayMemberAlreadyWritten = true
	}
	buffer.WriteString("]")

	fmt.Printf("- getHistoryForRecord returning:\n%s\n", buffer.String())

	return shim.Success(buffer.Bytes())
}

// ===========================================================================================
// cryptoVerify : Verifies signed message against public key
// Public Key of Authority:
// [48 78 48 16 6 7 42 134 72 206 61 2 1 6 5 43 129 4 0 33 3 58 0 4 21 162 242 84 40 78 13 26 160 33 97 191 210 22 152 134 162 66 12 77 221 129 138 60 74 243 198 34 102 209 14 48 16 2 98 96 172 47 170 216 228 169 103 121 153 100 84 111 33 13 106 42 46 227 52 91]
// ===========================================================================================
func cryptoVerify(hash []byte, publicKeyBytes []byte, r *big.Int, s *big.Int) (result bool) {
	fmt.Println("- Verifying ECDSA signature")
	fmt.Println("Message")
	fmt.Println(hash)
	fmt.Println("Public Key")
	fmt.Println(publicKeyBytes)
	fmt.Println("r")
	fmt.Println(r)
	fmt.Println("s")
	fmt.Println(s)

	publicKey, err := x509.ParsePKIXPublicKey(publicKeyBytes)
	if err != nil {
		fmt.Println(err.Error())
		return false
	}

	switch publicKey := publicKey.(type) {
	case *ecdsa.PublicKey:
		return ecdsa.Verify(publicKey, hash, r, s)
	default:
		return false
	}
}
