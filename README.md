# Switchboard: SSH Port Forwarding Interface

SSH is super cool! You can use it to easily yet securely access a remote system,
but did you know that you can also use it to forward ports between systems?

On the "standard" OpenSSH client, you can use the `-L` flag to forward a remote
port to a local machine. This is useful if, for example, you want to access a
website remotely that is usually only exposed to localhost. Alternatively, you
can use `-R` to forward a local port to a remote machine. This is useful if you
wanted to allow a system temporary access to a service through a firewall
without actually changing configurations.

However, the state of SSH port forwarding is not perfect. It can be difficult to
remember the order that the port nubmers go in without re-reading the man page.
It can also be tedious to manually re-type the same commands for commonly used
systems repeatedly. This can be simplified with shell scripts, but each host
requires its own process, meaning that you may have to have multiple port
forwards open at the same time.

Switchboard provies a simple TUI that helps you manage which port forwards are
currently active. It uses a native Go implementation of the SSH protocol, rather
than using the OpenSSH library. This means that you can leverage Go's excellent
cross compilation support to create a binary for whatever client you need to
target. Is your gateway an ARM system running Linux? switchboard has you covered. Is
your laptop running Windows? Switchboard works for that too.

## Usage

TODO

## Acknowledgements

Thanks to Stack Overflow user damick for making a great port forwarding
[example](https://stackoverflow.com/questions/21417223/simple-ssh-port-forward-in-golang).

## License

Copyright (C) 2026 Joseph Skubal

This program is free software: you can redistribute it and/or modify it under
the terms of the GNU General Public License as published by the Free Software
Foundation, either version 3 of the License, or (at your option) any later
version.

This program is distributed in the hope that it will be useful, but WITHOUT ANY
WARRANTY; without even the implied warranty of MERCHANTABILITY or FITNESS FOR A
PARTICULAR PURPOSE. See the GNU General Public License for more details.

You should have received a copy of the GNU General Public License along with
this program. If not, see https://www.gnu.org/licenses/.
