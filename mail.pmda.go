/*
 * Copyright (c) 2024 Gilles Chehade <gilles@poolp.org>
 *
 * Permission to use, copy, modify, and distribute this software for any
 * purpose with or without fee is hereby granted, provided that the above
 * copyright notice and this permission notice appear in all copies.
 *
 * THE SOFTWARE IS PROVIDED "AS IS" AND THE AUTHOR DISCLAIMS ALL WARRANTIES
 * WITH REGARD TO THIS SOFTWARE INCLUDING ALL IMPLIED WARRANTIES OF
 * MERCHANTABILITY AND FITNESS. IN NO EVENT SHALL THE AUTHOR BE LIABLE FOR
 * ANY SPECIAL, DIRECT, INDIRECT, OR CONSEQUENTIAL DAMAGES OR ANY DAMAGES
 * WHATSOEVER RESULTING FROM LOSS OF USE, DATA OR PROFITS, WHETHER IN AN
 * ACTION OF CONTRACT, NEGLIGENCE OR OTHER TORTIOUS ACTION, ARISING OUT OF
 * OR IN CONNECTION WITH THE USE OR PERFORMANCE OF THIS SOFTWARE.
 */

package main

import (
	"bufio"
	"crypto/rand"
	"flag"
	"fmt"
	"math/big"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const (
	EX_TEMPFAIL = 75
)

func maildir_mkdirs(maildir string) {
	for _, subdir := range []string{"new", "cur", "tmp"} {
		path := filepath.Join(maildir, subdir)
		if err := os.MkdirAll(path, 0700); err != nil {
			fmt.Fprintf(os.Stderr, "Error creating %s: %s\n", path, err)
			os.Exit(EX_TEMPFAIL)
		}
	}
}

func maildir_engine(maildir string) {
	maildir_mkdirs(maildir)
	maildir_mkdirs(filepath.Join(maildir, ".Error"))
	maildir_mkdirs(filepath.Join(maildir, ".Junk"))
	maildir_mkdirs(filepath.Join(maildir, ".List"))
	maildir_mkdirs(filepath.Join(maildir, ".Marketing"))
	maildir_mkdirs(filepath.Join(maildir, ".Transactional"))

	if extension := os.Getenv("EXTENSION"); extension != "" {
		subdir := filepath.Join(maildir, extension)
		if _, err := os.Stat(subdir); err == nil {
			maildir_mkdirs(subdir)
			maildir = subdir
		}
	}

	hostname, err := os.Hostname()
	if err != nil {
		hostname = os.Getenv("HOSTNAME")
		if hostname == "" {
			hostname = "localhost"
		}
	}

	nBig, err := rand.Int(rand.Reader, big.NewInt(0xffffffff))
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error generating random number: %s\n", err)
		os.Exit(EX_TEMPFAIL)
	}
	filename := fmt.Sprintf("%d.%08x.%s", time.Now().Unix(), uint32(nBig.Uint64()), hostname)

	pathname := filepath.Join(maildir, "tmp", filename)
	file, err := os.Create(pathname)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error creating %s: %s\n", pathname, err)
		os.Exit(EX_TEMPFAIL)
	}
	defer file.Close()

	scanner := bufio.NewScanner(os.Stdin)
	writer := bufio.NewWriter(file)

	isMarketing := false
	isError := false
	isJunk := false
	isList := false
	isHdr := true
	for scanner.Scan() {
		line := scanner.Text()
		fmt.Println(line)

		if isHdr && line == "" {
			isHdr = false
		} else if isHdr {
			if strings.ToLower(line) == "x-spam: yes" ||
				strings.ToLower(line) == "x-spam-flag: yes" {
				isJunk = true
			} else if strings.ToLower(line) == "precedence: bulk" {
				isMarketing = true
			} else if strings.ToLower(line) == "precedence: list" {
				isList = true
			} else if strings.ToLower(line) == "return-path: <>" {
				isError = true
			}
		}
		fmt.Fprintf(writer, "%s\n", line)
	}
	writer.Flush()

	if err := scanner.Err(); err != nil {
		fmt.Fprintf(os.Stderr, "Error reading from stdin: %s\n", err)
		os.Exit(EX_TEMPFAIL)
	}

	if isJunk {
		os.Rename(pathname, filepath.Join(maildir, ".Junk", "new", filename))
	} else if isMarketing {
		os.Rename(pathname, filepath.Join(maildir, ".Marketing", "new", filename))
	} else if isList {
		os.Rename(pathname, filepath.Join(maildir, ".List", "new", filename))
	} else if isError {
		os.Rename(pathname, filepath.Join(maildir, ".Error", "new", filename))
	} else {
		os.Rename(pathname, filepath.Join(maildir, "new", filename))
	}
}

// main is the entry point of the maildir delivery agent
func main() {
	flag.Parse()

	homedir := os.Getenv("HOME")
	if homedir == "" {
		fmt.Fprintf(os.Stderr, "HOME environment variable not set\n")
		os.Exit(EX_TEMPFAIL)
	}

	maildir := filepath.Join(homedir, "/Maildir")
	if flag.NArg() == 1 {
		maildir = flag.Arg(0)
	} else if flag.NArg() > 1 {
		fmt.Fprintf(os.Stderr, "Usage: %s [maildir]\n", os.Args[0])
		os.Exit(EX_TEMPFAIL)
	}

	maildir_engine(maildir)

	os.Exit(0)
}
