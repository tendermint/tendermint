Corruption
==========

Important step
--------------

Make sure you have a backup of the Tendermint data directory.

Possible causes
---------------

Remember that most corruption is caused by hardware issues:

- RAID controllers with faulty / worn out battery backup, and an unexpected power loss
- Hard disk drives with write-back cache enabled, and an unexpected power loss
- Cheap SSDs with insufficient power-loss protection, and an unexpected power-loss
- Defective RAM
- Defective or overheating CPU(s)

Other causes can be:

- Database systems configured with fsync=off and an OS crash or power loss
- Filesystems configured to use write barriers plus a storage layer that ignores write barriers. LVM is a particular culprit.
- Tendermint bugs
- Operating system bugs
- Admin error
  - directly modifying Tendermint data-directory contents

(Source: https://wiki.postgresql.org/wiki/Corruption)

WAL Corruption
--------------

If consensus WAL is corrupted at the lastest height and you are trying to start
Tendermint, replay will fail with panic.

Recovering from data corruption can be hard and time-consuming. Here are two approaches you can take:

1) Delete the WAL file and restart Tendermint. It will attempt to sync with other peers.
2) Try to repair the WAL file manually:

  1. Create a backup of the corrupted WAL file:

  .. code:: bash

      cp "$TMHOME/data/cs.wal/wal" > /tmp/corrupted_wal_backup

  2. Use ./scripts/wal2json to create a human-readable version

  .. code:: bash

      ./scripts/wal2json/wal2json "$TMHOME/data/cs.wal/wal" > /tmp/corrupted_wal

  3. Search for a "CORRUPTED MESSAGE" line.
  4. By looking at the previous message and the message after the corrupted one
       and looking at the logs, try to rebuild the message. If the consequent
       messages are marked as corrupted too (this may happen if length header
       got corrupted or some writes did not make it to the WAL ~ truncation),
       then remove all the lines starting from the corrupted one and restart
       Tendermint.

  .. code:: bash

      $EDITOR /tmp/corrupted_wal

  5. After editing, convert this file back into binary form by running:

  .. code:: bash

      ./scripts/json2wal/json2wal /tmp/corrupted_wal > "$TMHOME/data/cs.wal/wal"
