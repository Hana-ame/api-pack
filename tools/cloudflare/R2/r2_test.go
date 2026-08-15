// 纯纯debug用。。

package r2

import (
	"fmt"
	"io"
	"log"
	"os"
	"strings"
	"testing"
)

type wr struct {
	io.Reader
	io.Seeker
}

func (r *wr) Read(b []byte) (int, error) {
	log.Println("read w/buf", len(b))
	n, err := r.Reader.Read(b)
	fmt.Println("read: ", string(b[:n]), err)
	return n, err
}

type seeker struct {
	io.Seeker
}

func (s *seeker) Seek(offset int64, whence int) (int64, error) {
	return 0, fmt.Errorf("不样")
	fmt.Println("offset, whence", offset, whence)
	abs, err := s.Seeker.Seek(offset, whence)
	fmt.Println("abs,err", abs, err)
	return abs, err
}

func WR(r io.ReadSeeker) *wr {

	return &wr{Reader: r, Seeker: &seeker{r}}
}

type r struct {
	string
	int
}

func (r *r) Read(buf []byte) (int, error) {
	log.Println("r.Read called with buf:", len(buf))
	if r.int > 0 {
		return 0, io.EOF
	}
	n, err := strings.NewReader(string(r.string)).Read(buf)
	r.int += n
	return n, err
}

func R(s string) *r {
	return &r{string: s, int: 0}
}

func TestReadAll(t *testing.T) {
	r := R("test content")
	b, e := io.ReadAll(r)
	log.Println(string(b))
	log.Println(e)
	b, e = io.ReadAll(r)
	log.Println(string(b))
	log.Println(e)
	log.Println(r.Read([]byte("test")))
}

func TestStringsReader(t *testing.T) {
	r := strings.NewReader("test content")
	b, e := io.ReadAll(r)
	log.Println(string(b))
	log.Println(e)
	b, e = io.ReadAll(r)
	log.Println(string(b))
	log.Println(e)
	log.Println(r.Read([]byte("test")))
}

func TestReadWithNonEmptyBuffer(t *testing.T) {
	// r := R("test")
	r := strings.NewReader("test content")

	buf := make([]byte, 40)
	n, e := r.Read(buf)
	log.Println("Read bytes:", n, "Error:", e)
	log.Println("Buffer content:", string(buf))
	n, e = r.Read(buf)
	log.Println("Read bytes:", n, "Error:", e)
	log.Println("Buffer content:", string(buf))
	n, e = r.Read(buf)
	log.Println("Read bytes:", n, "Error:", e)
	log.Println("Buffer content:", string(buf))
}

func TestXxx(t *testing.T) {
	fmt.Println("为啥")
	r := strings.NewReader("test content")
	r.Read([]byte{1})

	b, _ := NewBucket(os.Getenv("R2_NAME"), os.Getenv("R2_ACCOUNT_ID"), os.Getenv("R2_ACCESS_KEY_ID"), os.Getenv("R2_ACCESS_KEY_SECRET"))
	if err := b.Upload("test.txt", R("123"), "application/x-www-urlencoded", -1); err != nil {
		t.Error(err)
	}
	// if err := b.Upload("test.txt", (f), "application/x-www-urlencoded"); err != nil {
	// 	t.Error(err)
	// }
}
