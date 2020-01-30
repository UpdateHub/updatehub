# updatehub [![Build Status](https://travis-ci.org/otavio/updatehub.svg?branch=next)](https://travis-ci.org/otavio/updatehub) [![Coverage Status](https://coveralls.io/repos/github/otavio/updatehub/badge.svg?branch=next)](https://coveralls.io/github/otavio/updatehub?branch=next) [![FOSSA Status](https://app.fossa.com/api/projects/git%2Bgithub.com%2Fotavio%2Fupdatehub.svg?type=shield)](https://app.fossa.com/projects/git%2Bgithub.com%2Fotavio%2Fupdatehub?ref=badge_shield)

UpdateHub provides a generic and safe Firmware Over-The-Air agent for
Embedded and Industrial Linux-based devices.

This repository is a fork from the official UpdateHub agent, exploring
the possibility of rewriting it on Rust. It is not close to completion and
should not be used in production yet.

For the official UpdateHub agent, please use the
https://github.com/UpdateHub/UpdateHub repository instead.

## Running tests

Some tests are marked as ignored because they require user previleges. There's 
a Vagrant file that can be used to run them. To run tests on the virtual machine run:

```Bash
vagrant up
vagrant ssh
sudo -i
cd /vagrant
cargo test
cargo test -- --ignored
```
