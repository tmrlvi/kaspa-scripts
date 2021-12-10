package main

import (
  "encoding/hex"
  "fmt"
  "os"
  "errors"

  "github.com/kaspanet/kaspad/util/bech32"
  "github.com/kaspanet/kaspad/infrastructure/network/rpcclient"
  "github.com/kaspanet/kaspad/app/appmessage"
)


const (
  prefix = "kaspa"
)

func printErrorAndExit(message string) {
	fmt.Fprintf(os.Stderr, fmt.Sprintf("%s\n", message))
	os.Exit(1)
}

func colorInfo(block *appmessage.RPCBlock, chain *appmessage.GetVirtualSelectedParentChainFromBlockResponseMessage) (int, string) {
  isRed := 0
  // If we found a child, we must be either red or blue
  for _, hash := range chain.AddedChainBlockHashes {
    for _, child := range block.VerboseData.ChildrenHashes {
      if child == hash {
        isRed = -1
        for _, redHash := range chain.RemovedChainBlockHashes {
          if redHash == block.VerboseData.Hash {
            isRed = 1
          }
        }
        return isRed, child
      }
    }
  }
  return isRed, ""
}

func getCoinBase(transactions []*appmessage.RPCTransaction) (*appmessage.RPCTransaction, error) {
  for i := range transactions {
    if len(transactions[i].Inputs) == 0 && len(transactions[i].Payload) > 0{
      return transactions[i], nil
    }
  }
  return nil, errors.New("Could not find coinbase")
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

func getBlockWithTransactions(client *rpcclient.RPCClient, hash string) *appmessage.RPCBlock {
  response, err := client.GetBlock(hash, true)
  if err != nil {
    printErrorAndExit(fmt.Sprintf("error getting block: %s", err))
  }
  return response.Block
}

func isAddressInTransactions(address []byte, transactions []*appmessage.RPCTransactionOutput) bool {
  for i := range transactions {
    tran_script, _ := hex.DecodeString(transactions[i].ScriptPublicKey.Script)
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

  client, err := rpcclient.NewRPCClient("localhost:16110")
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
  fmt.Println(fmt.Sprintf("Block %s", blockHash))
  fmt.Println(fmt.Sprintf("Mined by: %s (accroding to the payload).", ScriptAddressToAddress(script, version)))

  virtualChain, err := client.GetVirtualSelectedParentChainFromBlock(block.VerboseData.Hash)
  if err != nil {
    printErrorAndExit(fmt.Sprintf("error could not get virtual chain: %s", err))
  }
  isRed, selectedChild := colorInfo(block, virtualChain)

  if isRed == 1{
    fmt.Println("\u274c Block color is \033[31mred\033[0m.")
  } else if isRed == -1 {
    fmt.Println("\u2705 Block color is \033[34mblue\033[0m.")
  } else {
    fmt.Println("\u2754 Block color is not determined yet.")
  }


  if (selectedChild != ""){
    child_block := getBlockWithTransactions(client, selectedChild)
    child_coinbase, _ := getCoinBase(child_block.Transactions)
    if isAddressInTransactions(script, child_coinbase.Outputs) {
      fmt.Println(fmt.Sprintf("\u2705 Child's coinbase transaction %s contains the address.", child_coinbase.VerboseData.TransactionID))
    } else {
      fmt.Println(fmt.Sprintf("\u274c Child's coinbase transaction %s does not contain the address.", child_coinbase.VerboseData.TransactionID))
    }
  }

  /*if len(block.VerboseData.ChildrenHashes) == 0 {
    fmt.Println("The block does not have childern. Perhaps was not validated yet.")
  } else {
    for i := range block.VerboseData.ChildrenHashes {
      child_block := getBlockWithTransactions(client, block.VerboseData.ChildrenHashes[i])
      child_coinbase, _ := getCoinBase(child_block.Transactions)
      if isAddressInTransactions(script, child_coinbase.Outputs) {
        fmt.Println(fmt.Sprintf("\u2705 Child's coinbase transaction %s contains the address.", child_coinbase.VerboseData.TransactionID))
      } else {
        fmt.Println(fmt.Sprintf("\u274c Child's coinbase transaction %s does not contain the address.", child_coinbase.VerboseData.TransactionID))
      }
    }
  }*/
}
