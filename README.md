![UpdateHub logo](doc/updatehub.png)

---

UpdateHub is an enterprise-grade solution which makes simple to remotely update all your Linux-based devices in the field. It handles all aspects related to sending Firmware Over-the-Air (FOTA) updates with maximum security and efficiency, while you focus in adding value to your product.

To learn more about UpdateHub, check out our [documentation](https://docs.updatehub.io).

## Features

* **Yocto Linux support**: Integrate UpdateHub onto your existing Yocto based project
* **Scalable**: Send updates to one device, or one million
* **Reliability and robustness**: Automated rollback in case of update fail
* **Powerful API & SDK**: Extend UpdateHub to fit your needs

## UpdateHub Linux Agent

[![Build Status](https://travis-ci.org/UpdateHub/updatehub.svg?branch=v1)](https://travis-ci.org/updatehub/updatehub) [![Coverage Status](https://coveralls.io/repos/github/updatehub/updatehub/badge.svg?branch=v1)](https://coveralls.io/github/updatehub/updatehub?branch=v1)

This repository contains the UpdateHub Linux Agent, which can be run as system service in Yocto based images.

#### Building

Prerequisites:

* make
* libarchive-dev

```
$ make vendor
$ make
$ make test
```

## Contributing

UpdateHub is an open source project and we love to receive contributions from our community.
If you would like to contribute, please read our [contributing guide](CONTRIBUTING.md).

## License

UpdateHub Linux Agent is licensed under the GPLv2. See [COPYING](COPYING) for the full license text.

## Getting in touch

* Reach us on [Gitter](https://gitter.im/UpdateHub/community)
* All source code are in [Github](https://github.com/UpdateHub)
* Email us at [contact@updatehub.io](mailto:contact@updatehub.io)

