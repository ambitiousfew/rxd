# RxDaemon (RxD)
[![codecov](https://codecov.io/gh/ambitiousfew/rxd/branch/main/graph/badge.svg?token=3VTUQEX7HC)](https://codecov.io/gh/ambitiousfew/rxd)

A simple (alpha) reactive services daemon

> NOTE: RxD has been through another refactor to make it more flexible with the introduction of injectable "Service Managers" which are effectively supervisors of a given service. Intracom (pub/sub package) has become a subpackage of rxd.

RxD leverages [Intracom](https://github.com/ambitiousfew/rxd/v2/intracom) for internal comms which allows for the ability of your individual RxD services to subscribe interest to the lifecycle states of other RxD services running alongside each other. This means each service can be notified independently of what state another service is in. You can ultimately have any service "watch and react" to a state change of another service.

A good example to imagine here would be something like **ServiceA** that has subscribed interest in another service, **ServiceB**, where **ServiceB** happens to be a health check service. It might maintain a live TCP connection to an external service, run interval queries against a database engine, or health check a docker container, it doesnt really matter. The goal here is to NOT have alot of services individually doing their own health checks against the same resource because the more services you have the more checks you are potentially doing against the same resource which might be creating socket connections or doing file I/O not to mention the potential code duplication lines for each service to do their own check. Why not instead, write a service that can do the main logic of the check then signal using its own lifecycle states to anyone who is interested in that health check logic. RxD gives us that ability.


## Note about service managers like Systemd
In rxd when setting the WithReportAlive option on the daemon, this effectively causes the daemon during Start to launch a routine to interact with the underlying system service manager to report in with alive checks. For now this is purely meant for Systemd. This does also require you to set the appropriate configurations in your `.service` file. You would be required by systemd to set the `WatchdogSec` property.

This `WatchdogSec` property should be less for rxd because this is the amount of time systemd will wait to hear from your running daemon. After this time if your service has not reported in, systemd will consider it frozen/hung and will make attempts to stop or restart it.

By default, systemd has notify turned off. So it is possible to just not set or set a zero-value for the UsingReportAlive which will disable rxds notifier, by default RxD leaves this disabled.

```
[Unit]
Description=My Notifying Service

[Service]
Type=notify # WatchdogSec is required if this is set.
ExecStart=/path/to/my-service
NotifyAccess=main
WatchdogSec=10s  # Service must send a watchdog notification every 10 seconds, required it Type=notify is set.
```