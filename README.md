# transcript-packer
Downloads, organizes, and packs transcripts from the MLP Wiki into a JSON file.

### Usage

    transcript-packer.exe [-indexed] [min#] [max#]

Specifying `-indexed` means the episodes will be indexed by episode number instead of episode name. If no other arguments are provided, it will compile transcripts of seasons 1-7. If one number is provided, it will treat that number as the maximum season to download. If two numbers are provided, the first one is the minimum season and the second one is the maximum season (both inclusive)

### Examples

Download first two seasons, indexed by name:
    
    transcript-packer.exe 2 
    
Download seasons 3-6, indexed by number:

    transcript-packer.exe -indexed 3 6
