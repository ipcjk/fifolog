# fifolog
PoC, continuously read out a FIFO and automatically log+rotate file. This can be useful for logging data from a daemon that is not designed to log and rotate  to a file or has issues in doing so, e.g. icecast2, especially in docker environments.

## Usage

```bash
# Grep/Configure for the icecast2 log
grep accesslog icecast.xml
<accesslog>icecast2.log</accesslog>
# Start up fifolog
/icecast/fifolog -i /icecast/logs/icecast2.log -o /icecast/logs/access.log&
# Start up icecast2
/icecast/icecast -c /icecast/icecast.xml
# Now icecast2 should log to icecast2.log and fifolog logs/rotates to a daily log file.

```
