#!/bin/bash

# add watermark
composite -dissolve 20 -geometry +150+150  Boardsports_on-top-01-300x93.png windgraph.png watermarked.png

# composite the current and the watermarked graph
montage current.png watermarked.png -tile 2x1 -geometry +0+0 windreport_big.png 

# crush the file
pngcrush -q -m 7 -l 6 windreport_big.png windreport.png 

# Add ftp here
