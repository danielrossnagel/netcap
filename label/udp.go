/*
 * NETCAP - Traffic Analysis Framework
 * Copyright (c) 2017 Philipp Mieden <dreadl0ck [at] protonmail [dot] ch>
 *
 * THE SOFTWARE IS PROVIDED "AS IS" AND THE AUTHOR DISCLAIMS ALL WARRANTIES
 * WITH REGARD TO THIS SOFTWARE INCLUDING ALL IMPLIED WARRANTIES OF
 * MERCHANTABILITY AND FITNESS. IN NO EVENT SHALL THE AUTHOR BE LIABLE FOR
 * ANY SPECIAL, DIRECT, INDIRECT, OR CONSEQUENTIAL DAMAGES OR ANY DAMAGES
 * WHATSOEVER RESULTING FROM LOSS OF USE, DATA OR PROFITS, WHETHER IN AN
 * ACTION OF CONTRACT, NEGLIGENCE OR OTHER TORTIOUS ACTION, ARISING OUT OF
 * OR IN CONNECTION WITH THE USE OR PERFORMANCE OF THIS SOFTWARE.
 */

package label

import (
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/dreadl0ck/netcap"
	"github.com/dreadl0ck/netcap/types"
	"github.com/dreadl0ck/netcap/utils"
	"github.com/gogo/protobuf/proto"
	pb "gopkg.in/cheggaaa/pb.v1"
)

// UDP labels type NC_UDP.
func UDP(wg *sync.WaitGroup, file string, alerts []*SuricataAlert, outDir, separator, selection string) *pb.ProgressBar {
	var (
		fname       = filepath.Join(outDir, "UDP.ncap.gz")
		total       = netcap.Count(fname)
		labelsTotal = 0
		progress    = pb.New(int(total)).Prefix(utils.Pad(utils.TrimFileExtension(file), 25))
		outFileName = filepath.Join(outDir, "UDP_labeled.csv")
	)

	go func() {
		r, err := netcap.Open(fname)
		if err != nil {
			panic(err)
		}

		// read netcap header
		header := r.ReadHeader()
		if header.Type != types.Type_NC_UDP {
			panic("file does not contain UDP records: " + header.Type.String())
		}

		// outfile handle
		f, err := os.Create(outFileName)
		if err != nil {
			panic(err)
		}

		var (
			udp = new(types.UDP)
			fl  types.CSV
			pm  proto.Message
			ok  bool
		)
		pm = udp

		types.Select(udp, selection)

		if fl, ok = pm.(types.CSV); !ok {
			panic("type does not implement CSV interface")
		}

		// write header
		_, err = f.WriteString(strings.Join(fl.CSVHeader(), separator) + separator + "result" + "\n")
		if err != nil {
			panic(err)
		}

	read:
		for {
			err := r.Next(udp)
			if err == io.EOF || err == io.ErrUnexpectedEOF {
				break
			} else if err != nil {
				panic(err)
			}

			if UseProgressBars {
				progress.Increment()
			}

			var finalLabel string

			// Unidirectional UDP packets
			// checks if packet has a source or destination port matching an alert
			for _, a := range alerts {

				// must be a UDP packet
				if a.Proto == "UDP" &&

					// AND timestamp must match
					a.Timestamp == udp.Timestamp &&

					// AND destination port must match
					a.DstPort == int(udp.DstPort) &&

					// AND source port must match
					a.SrcPort == int(udp.SrcPort) {

					if CollectLabels {
						// only if it is not already part of the label
						if !strings.Contains(finalLabel, a.Classification) {
							if finalLabel == "" {
								finalLabel = a.Classification
							} else {
								finalLabel += " | " + a.Classification
							}
						}
						continue
					}

					// add label
					f.WriteString(strings.Join(udp.CSVRecord(), separator) + separator + a.Classification + "\n")
					labelsTotal++
					goto read
				}
			}

			if len(finalLabel) != 0 {
				// add final label
				f.WriteString(strings.Join(udp.CSVRecord(), separator) + separator + finalLabel + "\n")
				labelsTotal++
				goto read
			}

			// label as normal
			f.WriteString(strings.Join(udp.CSVRecord(), separator) + separator + "normal\n")
		}
		finish(wg, r, f, labelsTotal, outFileName, progress)
	}()
	return progress
}
