# photo_id_resizer
Resize Photo IDs using face recognition technology

**Description**

The program is used to resize large photo ID images which reside in a `source` directory and save them into a different
`destination` directory.  If an image file does not need to be resized (eg it is already smaller than `max height`), then the
file is simply copied from the `source` directory to the `destination` directory.  When image resizing occurs, this [content aware image resizing library](https://github.com/esimov/caire) is used with its face detection algorithm to avoid face deformation.

**Usage**

```
photo_id_resizer.exe: resize photo ID image files

  -a int
    	skip files older than X number of days. Ex: 0=do not skip any, 7=skip files older than a week
  -d string
    	destination directory
  -f string
    	path to 'facefinder' classification file (default: "facefinder")
  -h int
    	max image height, min size=10 (default: 500)
  -m string
    	regular expression to match files. Ex: jpg (default: "jpg|png")
  -s string
    	source directory
  -w int
    	number of files to process concurrently (default: # of CPU cores)
```

**Example**

    photo_id_resizer -s r:\photos -d r:\resized -f r:\facefinder -h 500 -m jpg -w 10 -a 30

Option | Explanation
-------|------------
-s r:\photos | source directory
-d r:\resized | destination directory
-f r:\facefinder | location of the 'facefinder' classification file
-h 500 | resize file if height is greater than 500 pixels, otherwise, just copy image to destination
-w 10 | process 10 images concurrently
-a 30 | skip files older then 30 days

**Acknowledgements**

* [Caire](https://github.com/esimov/caire) - a content aware image resizing library with face detection
* [facefinder](https://github.com/esimov/caire/blob/master/data/facefinder) -  the face finding classification file use by this program
* [Go Concurrency Patterns: Pipelines and cancellation](https://blog.golang.org/pipelines)

**License**

* [MIT License](LICENSE)
