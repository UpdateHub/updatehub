# updatehub [![Build Status](https://travis-ci.org/updatehub/updatehub.svg?branch=v1)](https://travis-ci.org/updatehub/updatehub) [![Coverage Status](https://coveralls.io/repos/github/updatehub/updatehub/badge.svg?branch=v1)](https://coveralls.io/github/updatehub/updatehub?branch=v1)

updatehub provides a generic and safe Firmware Over-The-Air agent for Embedded and
Industrial Linux-based devices.

Features
--------

* **6 install modes**

  * Copy: simple "mount", "copy", "umount" operation
  * Flash: flash-related operations using the binaries "flashcp", "nandwrite" and "flash_erase"
  * ImxKobs: imx-related operations using the "kobs-ng" binary
  * Raw: a "dd"-like mode (supports parameters like "skip", "seek", "count", etc)
  * Tarball: "mount", extract tarball and "umount"
  * Ubifs: ubifs-related operations using the binary "ubiupdatevol"

* **Automatic update discovery**

  * Configurable through files
  * Automatic query on a specified interval
  * Retry queries according to server policy
  * Don't loose its timing even when the device is rebooted or turned
    off for a long time

* **Conditional installation**

  * Install only if the target is different from the source
  * To decide what is different, can match string patterns or the
    entire target (through sha256sum)
  * Have presets for Linux kernel and U-boot to match versions

* **Active/Inactive configuration**

  * Using the Active/Inactive configuration, the device will contain 2
    installed systems in different partitions, 1 running (active) and
    1 inactive
  * The updates will be installed in the inactive partition
  * When an update installation fails, the device won't be bricked
    since the running system wasn't touched by the installation
  * When the update installation succeeds, the device reboots into the
    new installed system (which is now the active)

* **Pluggable**

  * The agent has a HTTP API that allows other applications to
    interact. This includes: trigger downloads, trigger installations,
    query status, query firmware metadata, query device information, etc.

  * Togethet with the HTTP API the agent supports several callback
    types: state change, error, validate, rollback. They are better
    explained below.

Prerequisites
--------

-  make
-  libarchive-dev

Build and test
--------

    make vendor
    make
    make test

updatehub Usage
--------

    ./bin/updatehub [flags]

    Flags:
          --debug   sets the log level to 'debug'
      -h, --help    help for updatehub
          --quiet   sets the log level to 'error'

updatehub Server Usage
--------

    ./bin/updatehub-server path [flags]

    Path:
      The directory path containing an uhupkg to be served.

    Flags:
          --debug   sets the log level to 'debug'
      -h, --help    help for updatehub-server
          --quiet   sets the log level to 'error'

updatehub Settings File
--------

Default path:

    /etc/updatehub.conf

Example file:

    [Polling]
    Interval=2h
    Enabled=false

    [Update]
    DownloadDir=/tmp/download
    SupportedInstallModes=mode1,mode2

    [Network]
    ServerAddress=http://addr:80

    [Firmware]
    MetadataPath=/usr/share/metadata

    [Storage]
    RuntimeSettingsPath=/var/lib/updatehub.conf

* **Polling**

  * Interval: the time interval on which each automatic poll will be
    done. ``Default: 1h``
  * Enabled: enable/disable the automatic polling. ``Default: enabled``

* **Update**

  * DownloadDir: the directory on which the update files will be
    downloaded. ``Default: /tmp``
  * SupportedInstallModes: the install modes supported by this
    target. ``Default: all ("dry-run", "copy", "flash", "imxkobs",
    "raw", "tarball", "ubifs")``
  
* **Network**

  * ServerAddress: the address used in the network requests. ``Default:
    https://api.updatehub.io``

* **Firmware**

  * MetadataPath: the directory on which are located the firmware
    metadata scripts. ``Default: /usr/share/updatehub``

* **Storage**

  * RuntimeSettingsPath: the file on which will be saved the runtime
    settings along reboots. ``Default: /var/lib/updatehub.conf``

HTTP API
--------

The HTTP API is detailed at: doc/agent-http.apib.

Callbacks
--------

Each callback is executed under certain circunstances:

* **State change**

This callback is executed before AND after every status change. When
it's executed before, the agent calls it like this:

    <callback> enter <state>

If this callback fails, the agent does NOT execute the <state> handle
and enter a error state. If this callback succeeds, proceeds as normal.

When it's executed after the <state>, the agent calls it like this:

    <callback> leave <state>

If this callback fails, the agent enter a error state. ``Default path:
/usr/share/updatehub/state-change-callback``

The output of both enter and leave actions are parsed to determine
transition state flow.

To cancel the current state transition, the callback must write
to stdout: ``cancel``.

* **Error**

This callback is executed whenever an error occurs. It's output is
ignored. ``Default path: /usr/share/updatehub/error-callback``

* **Validate**

This callback is executed whenever a new installation is booted. If
this callback succeeds the installation is validated and proceeds as
normal. If it fails, the agent forces a reboot into the previous
installation. ``Default path: /usr/share/updatehub/validate-callback``

* **Rollback**

This callback is executed whenever an installation validation
fails. After the failure, the agent reboots into the previous
installation and before entering it's normal execution, it executes
the rollback callback. ``Default path: /usr/share/updatehub/rollback-callback``
