# RxDaemon (RxD)
[![codecov](https://codecov.io/gh/ambitiousfew/rxd/branch/main/graph/badge.svg?token=3VTUQEX7HC)](https://codecov.io/gh/ambitiousfew/rxd)

A simple (alpha) reactive services daemon

> NOTE: Since RxD uses Intracom internally and because Intracom recently went through a large refactor from using mutex locks to no-locks, both Intracom and RxD should still be treated as an alpha project until further testing can be done.

RxD leverages [Intracom](https://github.com/ambitiousfew/intracom) for internal comms and allowing the ability for your individual RxD services to subscribe interest to the lifecycle states of other RxD services running alongside each other. This means each service can be notified independently of what state another service is in and since you control the logic of the lifecycle method and returning the next state you wish to enter. You can ultimately have your service "watch and react" to a state change of another service. 

A good example to imagine here would be something like **ServiceA** that has subscribed interest in another service, **ServiceB**, where **ServiceB** happens to be a health check service. It might maintain a live TCP connection to an external service, run interval queries against a database engine, or health check a docker container, it doesnt really matter. The goal here is to NOT have alot of services individually doing their own health checks against the same resource because the more services you have the more checks you are potentially doing against the same resource which might be creating socket connections or doing file I/O not to mention the potential code duplication lines for each service to do their own check. Why not instead, write a service that can do the main logic of the check then signal using its own lifecycle states to anyone who is interested in that health check logic. RxD gives us that ability.

