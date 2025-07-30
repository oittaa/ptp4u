# ptp4u
Scalable PTPv2.1 two-step unicast server implementation

Forked from: https://github.com/facebook/time/tree/main/ptp/ptp4u

## Run
Default arguments are good for most of the cases.
However, there is a lot of room for customisation:
```
/usr/local/bin/ptp4u -iface eth1 -workers 20 -monitoringport 1234
```
This will run ptp4u on eth1 with 20 send workers. Instance can be monitored on port 1234.

## Monitoring
By default ptp4u runs http server serving json monitoring data on port 8888. Ex:
```
$ curl -s localhost:8888 | jq
{
  "clockaccuracy": 33,
  "clockclass": 6,
  "drain": 0,
  "reload": 0,
  "rx.delay_req": 60,
  "subscriptions.delay_req": 1,
  "tx.announce": 60,
  "tx.sync": 60,
  "txts.missing": 0,
  "utcoffset_sec": 37,
  "worker.79.subscriptions": 1
}
```
This returns many useful metrics such as the number of active subscriptions, tx/rx stats etc.
