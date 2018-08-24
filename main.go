package main

import (
	"fmt"
	"context"

	core "github.com/daseinio/dasein-go-sdk/core"
	fsrepo "github.com/ipfs/go-ipfs/repo/fsrepo"
	cid "gx/ipfs/QmcZfnkapfECQGcLZaf9B79NRg7cRa9EnZh4LSbkCzwNvY/go-cid"
)

func main (){
	repo, err := fsrepo.Open("/Users/ggxxjj123/ipfs_test/ipfs2")
	if err != nil {
		fmt.Printf(err.Error())
	}

	nCfg := &core.BuildCfg{
		Repo:      repo,
		Permanent: true, // It is temporary way to signify that node is permanent
		Online:    true,
		ExtraOpts: map[string]bool{
			"mplex":  false,
		},
	}

	ipfsNode, err := core.NewNode(context.TODO(), nCfg)
	if err != nil {
		fmt.Printf(err.Error())
	}

	c, err := cid.Decode("QmafkFaZHH5zWoLupE5W9zkGsF8r6ShTxWGF7topbxXRnU")
	if err != nil {
		fmt.Println(err.Error())
	}

	block, err := ipfsNode.Exchange.GetBlock(context.Background(), c)
	if err != nil {
		fmt.Println(err.Error())
	} else {
		fmt.Println("CidString: ")
		fmt.Println(block.Cid().String())

		fmt.Println("RawData: ")
		fmt.Println(string(block.RawData()))
	}

}
