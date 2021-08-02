package main

import (
	"fmt"
	"io"
	"log"
	"os"
	"time"

	"github.com/alexflint/go-arg"
	"github.com/pion/webrtc/v3"
	"github.com/pion/webrtc/v3/pkg/media"
	"github.com/pion/webrtc/v3/pkg/media/oggreader"
	"github.com/pion/webrtc/v3/pkg/media/oggwriter"

	"github.com/evgeniy-klemin/webrtc-sip-client/softphone"
)

type Args struct {
	Count       int    `arg:"-c" default:"1" help:"Count instances"`
	Invite      string `arg:"-i" placeholder:"NUMBER" help:"Number for invite"`
	Username    string `default:"101"`
	Password    string `default:"101"`
	Domain      string `default:"local"`
	Transport   string `default:"ws"`
	Host        string `default:"192.168.100.10"`
	Path        string `help:"Path in server, for examples /webrtc/socket"`
	Port        uint16 `default:"5071"`
	SaveToFile  bool   `arg:"-s" default:"false" help:"Save media to file in ogg format --outfilename"`
	OutFileName string `placeholder:"FILENAME" default:"output.ogg"`
	InFileName  string `placeholder:"FILENAME" help:"Play ogg file in channel, example: --infilename input.ogg"`
	SRTPKey     string `default:"certs/dtls-srtp.pem" placeholder:"PATH"`
	SRTPCert    string `default:"certs/dtls-srtp.pub.pem" placeholder:"PATH"`
	Progress    bool   `arg:"-p" default:"false" help:"Display rtp progress"`
	Verbose     bool   `arg:"-v" default:"false" help:"Verbose"`
}

func main() {
	var args Args
	arg.MustParse(&args)

	cert := softphone.LoadCert(args.SRTPKey, args.SRTPCert)

	for i := 0; i < args.Count; i++ {
		go func() {
			softPhone(&args, cert)
		}()
		time.Sleep(time.Millisecond * 100)
	}
	select {}
}

func softPhone(args *Args, cert webrtc.Certificate) {
	phone := softphone.New(softphone.Options{
		Username:  args.Username,
		Password:  args.Password,
		Domain:    args.Domain,
		Transport: args.Transport,
		Host:      args.Host,
		Path:      args.Path,
		Port:      args.Port,
		Verbose:   args.Verbose,
	}, cert)
	phone.Register()

	inviteCount := 0

	phone.OnInvite = func(inviteMessage softphone.SipMessage) {
		inviteCount++
		if inviteCount > 1 {
			return
		}
		phone.Answer(inviteMessage)
	}

	phone.OnTrack = func(remote *webrtc.TrackRemote, local *webrtc.TrackLocalStaticSample) {
		fmt.Println("=======================================================================")
		fmt.Println("OnTrack")
		fmt.Println("=======================================================================")

		// Трансляция аудио из файла в коннект
		go func() {
			if local == nil || args.InFileName == "" {
				return
			}
			time.Sleep(5 * time.Second)
			fileName := args.InFileName
			f, err := os.OpenFile(fileName, os.O_RDONLY, 0600)
			if err != nil {
				panic(err)
			}
			defer f.Close()
			oggFile, _, err := oggreader.NewWith(f)
			if err != nil {
				panic(err)
			}
			var lastGranule uint64
			for {
				page, head, err := oggFile.ParseNextPage()
				if err != nil {
					if err == io.EOF {
						break
					}
					panic(err)
				}

				sampleCount := float64(head.GranulePosition - lastGranule)
				lastGranule = head.GranulePosition
				sampleDuration := time.Duration((sampleCount/8000)*1000) * time.Millisecond

				if args.Verbose {
					fmt.Printf("input: lastGranule=%d sampleDuration=%d ms\r", int(lastGranule), sampleDuration.Milliseconds())
				}

				if err := local.WriteSample(media.Sample{Data: page, Duration: sampleDuration}); err != nil {
					panic(err)
				}
				time.Sleep(sampleDuration)
			}
		}()

		var oggFile *oggwriter.OggWriter
		if args.SaveToFile {
			var err error
			oggFile, err = oggwriter.New(args.OutFileName, 8000, 1)
			if err != nil {
				panic(err)
			}
		}

		defer oggFile.Close()
		var startNum uint16
		var size int
		t1 := time.Now()
		for {
			rtp, _, err := remote.ReadRTP()
			if err != nil {
				log.Fatal(err)
			}
			if startNum == 0 {
				startNum = rtp.SequenceNumber
			}
			if args.Progress {
				size = size + len(rtp.Payload)
				num := rtp.SequenceNumber - startNum
				durationSec := time.Since(t1).Seconds()
				speedB := float64(size) / durationSec
				speedP := float64(num) / durationSec
				fmt.Printf(
					"packet №: %d, data: %d bytes, speed: %.0f bytes/sec, %.0f pack/sec, duration: %.1f sec\r",
					num, size, speedB, speedP, durationSec,
				)
			}
			if args.SaveToFile {
				if err := oggFile.WriteRTP(rtp); err != nil {
					panic(err)
				}
			}
		}
	}

	time.Sleep(time.Second * 2)

	if args.Invite != "" {
		phone.Invite(args.Invite)
	}

	select {}
}
