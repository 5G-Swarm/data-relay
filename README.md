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

| name          | key         | value       |
| ------------- | ----------- | ----------- |
| TCPKnownList  | hostID/key  | IP:port/key |
| TCPDestDict   | IP:port/key | destID      |
| TCPServerList | hostID/key  | IP:port/key |
| TCPSockets    | IP:port/key | conn        |
| TCPSendCnt    | IP:port/key | 0           |
| TCPPortsDict  |             |             |
| TCPAddrList   | ip:port     | key         |
|               |             |             |
