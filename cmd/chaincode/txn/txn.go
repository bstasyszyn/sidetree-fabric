/*
Copyright SecureKey Technologies Inc. All Rights Reserved.

SPDX-License-Identifier: Apache-2.0
*/

package main

import (
	"fmt"
	"runtime/debug"

	"github.com/trustbloc/sidetree-fabric/cmd/chaincode/cas"

	"github.com/hyperledger/fabric/core/chaincode/shim"
	pb "github.com/hyperledger/fabric/protos/peer"
)

var logger = shim.NewLogger("sidetreetxncc")

const (
	// Available function names
	writeContent = "writeContent"
	readContent  = "readContent"
	writeAnchor  = "writeAnchor"
	anchorBatch  = "anchorBatch"
	warmup       = "warmup"
	// collection is the name of the private data collection for storing content
	collection = "dcas"
	// anchor address prefix
	anchorAddrPrefix = "sidetreetxn_"
)

// funcMap is a map of functions by function name
type funcMap map[string]func(shim.ChaincodeStubInterface, [][]byte) pb.Response

// SidetreeTxnCC ...
type SidetreeTxnCC struct {
	functions funcMap
}

// new returns chaincode
func new() shim.Chaincode {

	cc := &SidetreeTxnCC{functions: make(funcMap)}

	cc.functions[writeContent] = cc.write
	cc.functions[readContent] = cc.read
	cc.functions[writeAnchor] = cc.writeAnchor
	cc.functions[anchorBatch] = cc.anchorBatch
	cc.functions[warmup] = cc.warmup

	return cc
}

// Init - nothing to do for now
func (t *SidetreeTxnCC) Init(stub shim.ChaincodeStubInterface) pb.Response {
	return shim.Success(nil)
}

// Invoke manages metadata lifecycle operations (writeContent, readContent)
func (t *SidetreeTxnCC) Invoke(stub shim.ChaincodeStubInterface) (resp pb.Response) {

	txID := stub.GetTxID()

	defer handlePanic(&resp)

	args := stub.GetArgs()
	if len(args) > 0 {
		// only display first arg (function), remaining args may contain client data, do not log them
		logger.Debugf("[txID %s] SidetreeTxnCC Arg[0]=%s", txID, args[0])
	}

	// Get function name (first argument)
	if len(args) < 1 {
		errMsg := "function name is required"
		logger.Debugf("[txID %s] %s", txID, errMsg)
		return shim.Error(errMsg)
	}

	functionName := string(args[0])
	function, valid := t.functions[functionName]
	if !valid {
		errMsg := fmt.Sprintf("Invalid invoke function [%s]. Expecting one of: %s", functionName, t.functions.String())
		logger.Debugf("[txID %s] %s", errMsg)
		return shim.Error(errMsg)
	}
	return function(stub, args[1:])
}

// writeContent will write content using cas client
func (t *SidetreeTxnCC) write(stub shim.ChaincodeStubInterface, args [][]byte) pb.Response {
	txID := stub.GetTxID()
	if len(args) < 1 || len(args[0]) == 0 {
		err := "missing content"
		logger.Debugf("[txID %s] %s", txID, err)
		return shim.Error(err)
	}

	client := cas.New(stub, collection)

	address, err := client.Write(args[0])
	if err != nil {
		errMsg := fmt.Sprintf("failed to write content: %s", err.Error())
		logger.Errorf("[txID %s] %s", txID, errMsg)
		return shim.Error(errMsg)
	}

	return shim.Success([]byte(address))
}

// readContent will read content using cas client
func (t *SidetreeTxnCC) read(stub shim.ChaincodeStubInterface, args [][]byte) pb.Response {
	txID := stub.GetTxID()
	if len(args) < 1 || len(args[0]) == 0 {
		errMsg := "missing content address"
		logger.Debugf("[txID %s] %s", txID, errMsg)
		return shim.Error(errMsg)
	}

	client := cas.New(stub, collection)

	address := string(args[0])
	payload, err := client.Read(address)
	if err != nil {
		errMsg := fmt.Sprintf("failed to read content: %s", err.Error())
		logger.Errorf("[txID %s] %s", txID, errMsg)
		return shim.Error(errMsg)
	}

	if payload == nil {
		return pb.Response{
			Status:  404,
			Message: "content not found",
		}
	}

	return shim.Success(payload)
}

// anchorBatch will store batch and anchor files using cas client and
// record anchor file address on the ledger in one call
func (t *SidetreeTxnCC) anchorBatch(stub shim.ChaincodeStubInterface, args [][]byte) pb.Response {
	txID := stub.GetTxID()

	if len(args) < 2 || len(args[0]) == 0 || len(args[1]) == 0 {
		errMsg := "batch and anchor files are required"
		logger.Debugf("[txID %s] %s", txID, errMsg)
		return shim.Error(errMsg)
	}

	client := cas.New(stub, collection)

	// write batch file
	_, err := client.Write(args[0])
	if err != nil {
		errMsg := fmt.Sprintf("failed to write batch content: %s", err.Error())
		logger.Errorf("[txID %s] %s", txID, errMsg)
		return shim.Error(errMsg)
	}

	// write anchor file
	anchorAddr, err := client.Write(args[1])
	if err != nil {
		errMsg := fmt.Sprintf("failed to write anchor content: %s", err.Error())
		logger.Errorf("[txID %s] %s", txID, errMsg)
		return shim.Error(errMsg)
	}

	// record anchor file address on the ledger (Sidetree Transaction)
	err = stub.PutState(anchorAddrPrefix+anchorAddr, []byte(anchorAddr))
	if err != nil {
		errMsg := fmt.Sprintf("failed to write anchor address: %s", err.Error())
		logger.Errorf("[txID %s] %s", txID, errMsg)
		return shim.Error(errMsg)
	}

	return shim.Success(nil)
}

// writeAnchor will record anchor file address on the ledger
func (t *SidetreeTxnCC) writeAnchor(stub shim.ChaincodeStubInterface, args [][]byte) pb.Response {
	txID := stub.GetTxID()
	if len(args) < 1 || len(args[0]) == 0 {
		errMsg := "missing anchor file address"
		logger.Debugf("[txID %s] %s", txID, errMsg)
		return shim.Error(errMsg)
	}

	anchorAddr := string(args[0])

	// record anchor file address on the ledger (Sidetree Transaction)
	err := stub.PutState(anchorAddrPrefix+anchorAddr, []byte(anchorAddr))
	if err != nil {
		errMsg := fmt.Sprintf("failed to write anchor address: %s", err.Error())
		logger.Errorf("[txID %s] %s", txID, errMsg)
		return shim.Error(errMsg)
	}

	return shim.Success(nil)
}

func (m funcMap) String() string {
	str := ""
	i := 0
	for key := range m {
		if i > 0 {
			str += ", "
		}
		i++
		str += fmt.Sprintf("\"%s\"", key)
	}
	return str
}

//nolint -- unused stub variable
func (t *SidetreeTxnCC) warmup(stub shim.ChaincodeStubInterface, args [][]byte) pb.Response {
	return shim.Success(nil)
}

// handlePanic handles a panic (if any) by populating error response
func handlePanic(resp *pb.Response) {
	if r := recover(); r != nil {

		logger.Errorf("Recovering from panic: %s", string(debug.Stack()))

		errResp := shim.Error("panic: check server logs")
		resp.Reset()
		resp.Status = errResp.Status
		resp.Message = errResp.Message
	}
}

func main() {
	err := shim.Start(new())
	if err != nil {
		fmt.Printf("Error starting SidetreeTxnCC chaincode: %s", err)
	}
}
