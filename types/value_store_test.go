package types

import (
	"bytes"
	"crypto/sha1"
	"fmt"
	"io"
	"testing"

	"github.com/attic-labs/noms/chunks"
	"github.com/attic-labs/noms/ref"
	"github.com/stretchr/testify/assert"
)

func TestReadValueBlobLeafDecode(t *testing.T) {
	assert := assert.New(t)

	blobLeafDecode := func(r io.Reader) Value {
		i := decode(r)
		return NewBlob(i.(io.Reader))
	}

	reader := bytes.NewBufferString("b ")
	v1 := blobLeafDecode(reader)
	bl1 := newBlobLeaf([]byte{})
	assert.True(bl1.Equals(v1))

	reader = bytes.NewBufferString("b Hello World!")
	v2 := blobLeafDecode(reader)
	bl2 := newBlobLeaf([]byte("Hello World!"))
	assert.True(bl2.Equals(v2))
}

func TestWriteValue(t *testing.T) {
	assert := assert.New(t)

	vs := NewTestValueStore()
	testEncode := func(expected string, v Value) ref.Ref {
		r := vs.WriteValue(v).TargetRef()

		// Assuming that MemoryStore works correctly, we don't need to check the actual serialization, only the hash. Neat.
		assert.EqualValues(sha1.Sum([]byte(expected)), r.Digest(), "Incorrect ref serializing %+v. Got: %#x", v, r.Digest())
		return r
	}

	// Encoding details for each codec is tested elsewhere.
	// Here we just want to make sure codecs are selected correctly.
	b := NewBlob(bytes.NewBuffer([]byte{0x00, 0x01, 0x02}))
	testEncode(string([]byte{'b', ' ', 0x00, 0x01, 0x02}), b)

	testEncode(fmt.Sprintf("t [%d,\"hi\"]", StringKind), NewString("hi"))
}

func TestWriteBlobLeaf(t *testing.T) {
	assert := assert.New(t)
	vs := NewTestValueStore()

	buf := bytes.NewBuffer([]byte{})
	b1 := NewBlob(buf)
	bl1, ok := b1.(blobLeaf)
	assert.True(ok)
	r1 := vs.WriteValue(bl1).TargetRef()
	// echo -n 'b ' | sha1sum
	assert.Equal("sha1-e1bc846440ec2fb557a5a271e785cd4c648883fa", r1.String())

	buf = bytes.NewBufferString("Hello, World!")
	b2 := NewBlob(buf)
	bl2, ok := b2.(blobLeaf)
	assert.True(ok)
	r2 := vs.WriteValue(bl2).TargetRef()
	// echo -n 'b Hello, World!' | sha1sum
	assert.Equal("sha1-135fe1453330547994b2ce8a1b238adfbd7df87e", r2.String())
}

func TestCheckChunksInCache(t *testing.T) {
	assert := assert.New(t)
	cs := chunks.NewTestStore()
	cvs := newLocalValueStore(cs)

	b := NewEmptyBlob()
	cs.Put(EncodeValue(b, nil))
	cvs.set(b.Ref(), (*presentChunk)(b.Type()))

	bref := NewTypedRefFromValue(b)
	assert.NotPanics(func() { cvs.checkChunksInCache(bref) })
}

func TestCacheOnReadValue(t *testing.T) {
	assert := assert.New(t)
	cs := chunks.NewTestStore()
	cvs := newLocalValueStore(cs)

	b := NewEmptyBlob()
	bref := cvs.WriteValue(b)
	r := cvs.WriteValue(bref)

	cvs2 := newLocalValueStore(cs)
	v := cvs2.ReadValue(r.TargetRef())
	assert.True(bref.Equals(v))
	assert.True(cvs2.isPresent(b.Ref()))
	assert.True(cvs2.isPresent(bref.Ref()))
}

func TestHintsOnCache(t *testing.T) {
	assert := assert.New(t)
	cvs := newLocalValueStore(chunks.NewTestStore())

	bs := []Blob{NewEmptyBlob(), NewBlob(bytes.NewBufferString("f"))}
	l := NewList()
	for _, b := range bs {
		bref := cvs.WriteValue(b)
		l = l.Append(bref)
	}
	r := cvs.WriteValue(l)

	v := cvs.ReadValue(r.TargetRef())
	if assert.True(l.Equals(v)) {
		l = v.(List)
		bref := cvs.WriteValue(NewBlob(bytes.NewBufferString("g")))
		l = l.Insert(0, bref)

		hints := cvs.checkChunksInCache(l)
		if assert.Len(hints, 1) {
			_, present := hints[v.Ref()]
			assert.True(present)
		}
	}
}