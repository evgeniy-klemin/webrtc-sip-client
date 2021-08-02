# WebRTC SIP client on golang for FreeSwitch

> WebRTC SIP client for imitate webrtc client from browser.

Tested only with FreeSwitch 1.10 webrtc server.
Codec OPUS with 8000hz bandwith.


## Use cases

* Integration/functional tests webrtc server in CICD
* Stress test stage/production webrtc servers
* As client in development process

## Usage

### Send invite to wss://webrtc.site.com/webrtc with concurency 10

```bash
go run main.go --host webrtc.site.com --invite 0000 --transport wss --port 443 --path /webrtc -c 10
```

### Connect to wss://webrtc.site.com/webrtc and wait invite from webrtc server

```bash
go run main.go --host webrtc.site.com --transport wss --port 443 --path /webrtc -c 1
```


### Arguments

```bash
Usage: main [--count COUNT] [--invite NUMBER] [--username USERNAME] [--password PASSWORD] [--domain DOMAIN] [--transport TRANSPORT] [--host HOST] [--path PATH] [--port PORT] [--savetofile] [--outfilename FILENAME] [--infilename FILENAME] [--srtpkey PATH] [--srtpcert PATH] [--progress] [--verbose]

Options:
  --count COUNT, -c COUNT
                         Count instances [default: 1]
  --invite NUMBER, -i NUMBER
                         Number for invite
  --username USERNAME [default: 101]
  --password PASSWORD [default: 101]
  --domain DOMAIN [default: local]
  --transport TRANSPORT [default: ws]
  --host HOST [default: 192.168.100.10]
  --path PATH            Path in server, for examples /webrtc/socket
  --port PORT [default: 5071]
  --savetofile, -s       Save media to file in ogg format --outfilename [default: false]
  --outfilename FILENAME [default: output.ogg]
  --infilename FILENAME
                         Play ogg file in channel, example: --infilename input.ogg
  --srtpkey PATH [default: certs/dtls-srtp.pem]
  --srtpcert PATH [default: certs/dtls-srtp.pub.pem]
  --progress, -p         Display rtp progress [default: false]
  --verbose, -v          Verbose [default: false]
  --help, -h             display this help and exit
```
