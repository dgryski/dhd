package main

import (
	"flag"
	"fmt"
	"io"
	"log"
	"net"
	"os"
	"strings"
	"unicode"
)

type formatter struct {
	w      io.Writer
	offset uint
	prefix string
}

func min(a, b int) int {
	if a < b {
		return a
	}

	return b
}

func (f *formatter) Write(p []byte) (int, error) {
	for i := 0; i < len(p); i += 16 {
		m := min(len(p), i+16)
		f.format(p[i:m])
		f.offset += uint(m - i)
	}
	return len(p), nil
}

func (f *formatter) format(buf []byte) {

	s := fmt.Sprintf("%s%08x:  ", f.prefix, f.offset)

	if len(buf) == 16 {
		/* optimize for the most common case */
		s += fmt.Sprintf("%02x %02x %02x %02x  "+
			"%02x %02x %02x %02x  "+
			"%02x %02x %02x %02x  "+
			"%02x %02x %02x %02x ",
			buf[0], buf[1], buf[2], buf[3],
			buf[4], buf[5], buf[6], buf[7],
			buf[8], buf[9], buf[10], buf[11],
			buf[12], buf[13], buf[14], buf[15])
	} else {

		for i := 0; i < len(buf); i++ {
			if i != 0 && ((i % 4) == 0) {
				s += fmt.Sprintf(" ")
			}
			s += fmt.Sprintf("%02x ", buf[i])
		}

		for i := len(buf); i < 16; i++ {
			if i != 0 && ((i % 4) == 0) {
				s += fmt.Sprintf(" ")
			}
			s += fmt.Sprintf("   ")
		}
	}

	buf2 := strings.Map(func(r rune) rune {
		if unicode.IsPrint(r) && !unicode.IsSpace(r) {
			return r
		}
		return '.'
	}, string(buf[:]))

	s += fmt.Sprintf("   |%s|\n", string(buf2[:]))
	f.w.Write([]byte(s))
}

func main() {

	proxy := flag.String("p", "", "proxy line -- <lport>:<rhost>:<rport>")

	flag.Parse()

	// provided a proxy line
	if *proxy != "" {
		pieces := strings.Split(*proxy, ":")
		dst := pieces[1] + ":" + pieces[2]

		fin := &formatter{os.Stdout, 0, "<"}
		fout := &formatter{os.Stdout, 0, ">"}

		ln, e := net.Listen("tcp", ":"+pieces[0])
		if e != nil {
			log.Fatal("listen error:", e)
		}

		log.Println("tcp server starting")

		for {
			lconn, err := ln.Accept()
			if err != nil {
				log.Println(err)
				continue
			}

			go func(lconn net.Conn) {
				tl := io.TeeReader(lconn, fout)
				rconn, err := net.Dial("tcp", dst)
				if err != nil {
					log.Println("error connectiong to", dst, ":", err)
					lconn.Close()
					return
				}
				tr := io.TeeReader(rconn, fin)
				go func(rconn io.WriteCloser, tl io.Reader) { io.Copy(rconn, tl); rconn.Close() }(rconn, tl)
				go func(lconn io.WriteCloser, tr io.Reader) { io.Copy(lconn, tr); lconn.Close() }(lconn, tr)
			}(lconn)
		}
	}

	fout := &formatter{os.Stdout, 0, ""}

	var fin io.Reader

	// process stdin
	if flag.NArg() == 0 {
		fin = os.Stdin
	} else {
		fname := flag.Arg(0)
		var err error
		fin, err = os.Open(fname)
		if err != nil {
			log.Fatal(err)
			return
		}
	}

	io.Copy(fout, fin)
}
