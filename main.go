package main

import (
	"flag"
	"io"
	"log"
	"net"
	"os"
	"strings"
	"time"
)

type formatter struct {
	w      io.Writer
	offset uint
	prefix string
	tstamp bool
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

var hex = []byte("0123456789abcdef")

func (f *formatter) format(buf []byte) {

	// prefix addr:_(hex dump)+spaces+space+bar+chars+bar+newline

	// our line is 83 characters of formatting

	plen := len(f.prefix)

	if f.tstamp {
		plen += len(time.Stamp)
	}

	llen := 83 + plen

	ptr := 0

	line := make([]byte, llen)
	if f.tstamp {
		s := time.Now().Format(time.Stamp)
		copy(line, []byte(s))
		ptr += len(time.Stamp)
	}

	copy(line[ptr:], []byte(f.prefix))
	ptr += len(f.prefix)

	offs := f.offset

	ptr += 8

	line[ptr] = ':'
	ptr--

	for offs > 0 {
		line[ptr] = hex[offs&0x0f]
		ptr--
		offs >>= 4
	}

	for ptr >= plen {
		line[ptr] = '0'
		ptr--
	}

	ptr = plen + 9
	line[ptr] = ' '
	ptr++

	for i, b := range buf {
		if i%4 == 0 {
			line[ptr] = ' '
			ptr++
		}

		line[ptr] = hex[b>>4]
		ptr++
		line[ptr] = hex[b&0x0f]
		ptr++
		line[ptr] = ' '
		ptr++

	}

	// fill in rest of line
	for i := len(buf); i < 16; i++ {
		if i%4 == 0 {
			line[ptr] = ' '
			ptr++
		}

		line[ptr] = ' '
		ptr++
		line[ptr] = ' '
		ptr++
		line[ptr] = ' '
		ptr++
	}

	line[ptr] = ' '
	ptr++
	line[ptr] = ' '
	ptr++
	line[ptr] = '|'
	ptr++

	for _, v := range buf {
		if v > 32 && v < 127 {
			line[ptr] = v
		} else {
			line[ptr] = '.'
		}
		ptr++
	}

	line[ptr] = '|'
	ptr++

	line[ptr] = '\n'
	ptr++

	f.w.Write(line[:ptr])
}

func main() {

	proxy := flag.String("p", "", "proxy line -- <lport>:<rhost>:<rport>")
	tstamps := flag.Bool("t", false, "add time-stamps when proxying")

	flag.Parse()

	// provided a proxy line
	if *proxy != "" {
		pieces := strings.Split(*proxy, ":")
		dst := pieces[1] + ":" + pieces[2]

		fprefix := "<= "
		tprefix := "=> "

		if *tstamps {
			fprefix = " <= "
			tprefix = " => "
		}

		fin := &formatter{os.Stdout, 0, fprefix, *tstamps}
		fout := &formatter{os.Stdout, 0, tprefix, *tstamps}

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

	if *tstamps {
		log.Println("-t only applies when proxying, ignoring")
	}

	fout := &formatter{os.Stdout, 0, "", false}

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
