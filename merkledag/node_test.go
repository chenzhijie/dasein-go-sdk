package merkledag_test

import (
	"bytes"
	"testing"

	. "github.com/daseinio/dasein-go-sdk/merkledag"

	ipld "gx/ipfs/Qme5bWv7wtjUNGsK2BNGVUFPKiuxWrsqrtvYwCLRw8YFES/go-ipld-format"
)

func TestRemoveLink(t *testing.T) {
	nd := &ProtoNode{}
	nd.SetLinks([]*ipld.Link{
		{Name: "a"},
		{Name: "b"},
		{Name: "a"},
		{Name: "a"},
		{Name: "c"},
		{Name: "a"},
	})

	err := nd.RemoveNodeLink("a")
	if err != nil {
		t.Fatal(err)
	}

	if len(nd.Links()) != 2 {
		t.Fatal("number of links incorrect")
	}

	if nd.Links()[0].Name != "b" {
		t.Fatal("link order wrong")
	}

	if nd.Links()[1].Name != "c" {
		t.Fatal("link order wrong")
	}

	// should fail
	err = nd.RemoveNodeLink("a")
	if err != ipld.ErrNotFound {
		t.Fatal("should have failed to remove link")
	}

	// ensure nothing else got touched
	if len(nd.Links()) != 2 {
		t.Fatal("number of links incorrect")
	}

	if nd.Links()[0].Name != "b" {
		t.Fatal("link order wrong")
	}

	if nd.Links()[1].Name != "c" {
		t.Fatal("link order wrong")
	}
}

func TestNodeCopy(t *testing.T) {
	nd := &ProtoNode{}
	nd.SetLinks([]*ipld.Link{
		{Name: "a"},
		{Name: "c"},
		{Name: "b"},
	})

	nd.SetData([]byte("testing"))

	ond := nd.Copy().(*ProtoNode)
	ond.SetData(nil)

	if nd.Data() == nil {
		t.Fatal("should be different objects")
	}
}

func TestJsonRoundtrip(t *testing.T) {
	nd := new(ProtoNode)
	nd.SetLinks([]*ipld.Link{
		{Name: "a"},
		{Name: "c"},
		{Name: "b"},
	})
	nd.SetData([]byte("testing"))

	jb, err := nd.MarshalJSON()
	if err != nil {
		t.Fatal(err)
	}

	nn := new(ProtoNode)
	err = nn.UnmarshalJSON(jb)
	if err != nil {
		t.Fatal(err)
	}

	if !bytes.Equal(nn.Data(), nd.Data()) {
		t.Fatal("data wasnt the same")
	}

	if !nn.Cid().Equals(nd.Cid()) {
		t.Fatal("objects differed after marshaling")
	}
}
