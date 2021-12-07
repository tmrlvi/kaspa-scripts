package main

import (
  "encoding/hex"
  "encoding/json"
  //"encoding/binary"
  "fmt"
  "os"
  "errors"

  "github.com/kaspanet/kaspad/util/bech32"
  "github.com/kaspanet/kaspad/infrastructure/network/rpcclient/grpcclient"
)

const (
  prefix = "kaspa"
)

type RPCError struct {
  Message string `json:"message"`
}

type RPCBlock struct {
  Transactions []RPCTransaction `json:"transactions"`
  VerboseData RpcBlockVerboseData `json:"verboseData"`
}

type RpcBlockVerboseData struct {
  ChildrenHashes []string `json:"childrenHashes"`
}

type RPCTransaction struct {
  Payload string `json:"payload"`
  Inputs []RPCTransactionInput `json:"inputs"`
  Outputs []RPCTransactionOutput `json:"outputs"`
  VerboseData RpcTransactionVerboseData `json:"verboseData"`
}

type RPCTransactionInput struct {
}

type RPCTransactionOutput struct {
  ScriptPublicKey RpcScriptPublicKey `json:"scriptPublicKey"`
}

type RpcTransactionVerboseData struct {
  TransactionId string `json:"transactionId"`
}

type RpcScriptPublicKey struct {
  ScriptPublicKey string `json:"scriptPublicKey"`
}

type GetBlockRequestMessage struct {
  Hash string `json:"hash"`
  IncludeTransactions bool `json:"includeTransactions"`
}

type GetBlockResponseMessage struct {
  Block *RPCBlock `json:"block"`
  Error *RPCError `json:"error,omitempty"`
}

type Message struct {
  GetBlockRequest *GetBlockRequestMessage `json:"getBlockRequest,omitempty"`
  GetBlockResponse *GetBlockResponseMessage `json:"getBlockResponse,omitempty"`
}


func printErrorAndExit(message string) {
	fmt.Fprintf(os.Stderr, fmt.Sprintf("%s\n", message))
	os.Exit(1)
}

func getCoinBase(transactions []RPCTransaction) (RPCTransaction, error) {
  for i := range transactions {
    if len(transactions[i].Inputs) == 0 && len(transactions[i].Payload) > 0{
      return transactions[i], nil
    }
  }
  return RPCTransaction{}, errors.New("Could not find coinbase")
}

func payloadToScriptAddressAndVersion(prefix, payloadHex string) ([]byte, byte) {
  payload, err := hex.DecodeString(payloadHex)
  if err != nil {
    printErrorAndExit(fmt.Sprintf("error decoding payload hex: %s", err))
  }
  //version, _ := binary.LittleEndian.Uint16(data[16:18])
  version := payload[16] // They seem to require version to be 1 byte though they give it 2
  length :=  payload[18]
  script := payload[19:19+length]
  //extra := payload[19+length:]
  return script, version
}

func ScriptAddressToAddress(script []byte, version byte) string {
  address := []byte{}
  if script[0] < 0x76 {
    address_size := script[0] // - (OpData1 - 1), which = 0. We take care only for the case that len(address) < 0x76
    address = script[1:address_size+1]
  } else {
    printErrorAndExit("Complex parsing of script is not implemented")
  }
  return bech32.Encode(prefix, address, byte(version))
}

func getBlockWithTransactions(client *grpcclient.GRPCClient, hash string) *RPCBlock {
  request := Message{GetBlockRequest: &GetBlockRequestMessage{Hash: hash, IncludeTransactions: true}}
  requestJson, err := json.Marshal(request)
  if err != nil {
    printErrorAndExit(fmt.Sprintf("error encoding JSON: %s", err))
  }

  responseString, err := client.PostJSON(string(requestJson))
  if err != nil {
    printErrorAndExit(fmt.Sprintf("error posting the request to the RPC server: %s", err))
  }
  if !json.Valid([]byte(responseString)) {
    printErrorAndExit(fmt.Sprintf("returned json is invalid"))
  }

  var response Message
  err = json.Unmarshal([]byte(responseString), &response)
  if err != nil {
    printErrorAndExit(fmt.Sprintf("error parsing repsonse: %s", err))
  }
  return response.GetBlockResponse.Block
}

func isAddressInTransactions(address []byte, transactions []RPCTransactionOutput) bool {
  for i := range transactions {
    tran_script, _ := hex.DecodeString(transactions[i].ScriptPublicKey.ScriptPublicKey)
    if string(tran_script) == string(address) {
      return true
    }
  }
  return false
}

func main() {
  //TODO: check len pf os.Args
  if len(os.Args) != 2 {
    printErrorAndExit(fmt.Sprintf("usage: %s <hash>", os.Args[0]))
  }
  blockHash := os.Args[1]
  //prefix := os.Args[2] // kaspa

  client, err := grpcclient.Connect("localhost:16110")
  if err != nil {
    printErrorAndExit(fmt.Sprintf("error connecting to the RPC server: %s", err))
  }
  defer client.Disconnect()


  block := getBlockWithTransactions(client, blockHash)
  if block == nil {
    printErrorAndExit(fmt.Sprintf("Block not found"))
  }
  coinbaseTran, err := getCoinBase(block.Transactions)
  if err != nil {
    printErrorAndExit(fmt.Sprintf("error while search for coinbase: %s", err))
  }
  script, version := payloadToScriptAddressAndVersion(prefix, coinbaseTran.Payload)
  fmt.Println(fmt.Sprintf("Block %s.", blockHash))
  fmt.Println(fmt.Sprintf("Mined by: %s (accroding to the payload).", ScriptAddressToAddress(script, version)))
  if len(block.VerboseData.ChildrenHashes) == 0 {
    fmt.Println("The block does not have childern. Perhaps was not validated yet.")
  } else {
    for i := range block.VerboseData.ChildrenHashes {
      child_block := getBlockWithTransactions(client, block.VerboseData.ChildrenHashes[i])
      child_coinbase, _ := getCoinBase(child_block.Transactions)
      if isAddressInTransactions(script, child_coinbase.Outputs) {
        fmt.Println(fmt.Sprintf("\u2714 Child's coinbase transaction %s contains the address.", child_coinbase.VerboseData.TransactionId))
      } else {
        fmt.Println(fmt.Sprintf("\u274c Child's coinbase transaction %s does not contain the address.", child_coinbase.VerboseData.TransactionId))
      }
    }
  }
}
