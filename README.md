# data-relay

## how-to-use

Put the whole folder into *DiSLAM-Comm*'s parent path

like this:

> parent/
>
> -- data-relay
>
> -- DiSLAM-Comm
>
> -- some-other-files



Then build the sever:

```bash
$ go build
```

Finally, run the data-relay:

```bash
$ ./data_relay
```


## Dict info
| name          | key        | value      |
| ------------- | ---------- | ---------- |
| AddressMap    | hostID/key | IP:port    |
| DestMap       | hostID/key | destID/key |
| TCPServerList | hostID/key | IP:port    |
| TCPSockets    | hostID/key | conn       |
| TCPSendCnt    | hostID/key | 0          |