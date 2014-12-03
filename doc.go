/*
Checksum and TimeMachine verification utility to combat data rot on your hard drive.  For an excellent overview of what data rot is, take a look here: http://en.m.wikipedia.org/wiki/Data_degradation

sumcheck keeps a database of checksums of all the files in your filesystem.  When started, it will check all the known files against their previous checksums, and store new checksums for files it hasn't seen.  It can also compare the files to a backup copy, and for Mac users, it can automatically find your TimeMachine backup and compare against that.

To find out about my motivation for writing this, see Microsoft's paper at http://research.microsoft.com/pubs/64599/tr-2005-166.pdf on the vulnerability of data stored on hard drives.  tl;dr: The data on your hard drive can change spontaneously-- when you go back to read a file, a bit may have flipped without triggering any errors from the operating system.  This used to be considered quite rare, but as we have started storing more and more data, the bit error rates have remained about the same.  This means that it is getting more and more likely that users are suffering from this problem wihtout knowing it.  The best way to combat this is to use a filesystem that checksums your data, such as ZFS or BTRFS.  But for those who don't have the option, this little utility gives you a way to detect the problem, and if you have backups, decide which version of the file is correct.
*/
package main
