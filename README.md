# MediaManager
Database & Web interface for managing various media files

###### I need to write more documentation.

## Usage & Quickstart

Calling the program will show all arguments

### Create database & import files
To create a databse & import files use `mediamanager import <Database path>`, `-f` or `--importfiles` will import a given file and `-d` or `--importidirs` will import all valid files in the tree into the database, both of these arguments can be repeated any number of times.

You can use `--tag` to add a tag to all imported files, `-H` or `--addhash` to add a SHA-256 hash and `-S` or `--addsize` to add size to each file (This will increase import time)

You can also use a JSON file as config with `--importjson`, the format is below
```json
{
    // --addhash & --addsize
    "AddFileInfo": false,
    // -d, --importdirs
    "Dirs": {
        // Abs paths
        "MyAbsPath": {
            // Tags to add to every file in the path
            "Tags": [
                "MyTag1"
            ]
        }
    },
    // -f, --importfiles
    "Files": {
        // Abs path
        "MyAbsFile": {
            "Tags": [
                "MyTag2"
            ]
        }
    }
}
```

### Running
To run on a server run `mediamanager web <Database path>` by default this will run on (LocalAddress):5555

You can change the address using `-a (IP):(PORT)`

To add TLS support [generate .crt and .key files](https://serverfault.com/a/224127), then use `--cert (.crt file path)` and `--key (.key file path)`

To add login data use `--authconfig <Config path>`, the config is formatted as 

```json
{
    "Username": {
        "Password": "Password"
    }
    // Can repeat any number of times
}
```

## Supported File Types
Depends on browser support, by default imported files are 

### Images
- jpg
- png
- jpeg
- gif
### Videos
- webm
- mp4
- mpv
- m4v
### Audio
- mp3
- flac
- wav

You can add more in `util.go` `isImportableFile` function

# Database control
You can use the databbase subcommand for easy database control, you can also just open the database in any database browser you'd like.

## Add hashes & remove non unique files
To add hashes do

`mediamanager database <Database path> --selectnohash --updatehash`

This will attempt to add hashes to any file that doesn't already have them, and print any file that fails to have a hash added, the reason is that the file doesn't exist or if the hash already exists in the database.

We can then run

`mediamanager database <Database path> --selectnohash --remove`

to remove any file from the database that failed to hash, if you'd like to remove the files from disk as well you can add `--removefromdisk`