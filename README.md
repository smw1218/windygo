# windygo
Reading weather readings from a Davis Vantage Vue using ethernet.  Built specifically for reporting wind at [Boardsports California](http://boardsportscalifornia.com/weather1/alameda-weather) in Alameda, CA.
There's a bunch of stuff in here that's kinda specific to them.  Wind reports were my priority so not everything that the weather station does is implemented.  Running this app interferes with uploading to Davis' Weatherlink site.

Here's the [Vantage Spec](http://www.davisnet.com/support/weather/download/VantageSerialProtocolDocs_v261.pdf) that has the details about the wire protocol.

## Example Output
![Wind Graph](./example.png?raw=true)

## Features

- Reads "LOOP" records that the station produces every ~2s and creates summaries every 1, 5 and 10 minutes
- Saves summaries to mysql
- Creates a png report using gnuplot and ImageMagick
- Runs on a raspberry pi

## Installation
Dependencies: 

     sudo apt-get install gnuplot imagemagick mariadb-server fonts-roboto golang ftp git pngcrush

### Building
It's ok to just install go in the home directory.  Set you GOROOT and PATH to point to your new install:
  
     mkdir -p go go/src go/bin go/pkg
     export GOPATH=$(pwd)/go
     go get github.com/smw1218/windygo
     go install github.com/smw1218/windygo

If everything went well, the binary should be in gopath/bin.

### Fonts

I use Roboto and RobotoCondensed (Google Fonts) for the gnuplots and ImageMagick.  These need to installed for them to work. ImageMagick requires an extra xml file arrows/fonts.xml which needs to be copied into /etc/ImageMagick-6:

     sudo cp arrows/fonts.xml /etc/ImageMagick-6
  
Edit /etc/ImageMagick-6/type.xml and add this line between `<typemap>` and `</typemap>`:

     <include file="fonts.xml" />

I also created a custom font using [fontcustom](http://fontcustom.com/). It provides all the compass directions.  The font is in arrows/CompassArrows/CompassArrows.ttf and must be copied to a system font directory (for some reason on raspberry pi, gnuplot did not work with just setting GDFONTPATH):

     sudo cp arrows/CompassArrows/CompassArrows.ttf /usr/share/fonts/truetype/freefont/

### Mysql/MariaDB
You have to create the database for windygo by hand, but it will do the rest. Launch mysql as root and create it:

    mysql -u root < $GOPATH/src/github.com/smw1218/windygo/mysql_setup.sql

I didn't implement using a password, if someone files an issue I'll do it.

### Running

     windygo -h <ip address of your vantage>:22222

## Why?
Didn't I know about [weewx](http://www.weewx.com/) or [wview](http://www.wviewweather.com/)?  I looked at both, but the data I wanted from either one seemed difficult to get setup (though probably not as difficult as writing this).  The hard part is around the reports.  I wanted to get an update report every minute but the built in summaries for the Vantage Vue are 5 minutes minimum.  Both weewx and wview tie their report interval to the wether station so I couldn't get more frequent updates.  

Also, I thought it would be a fun go project.

## Bugs
I implemented the DMP and DMPAFT commands but both seem to have issues. The DMP returns several pages ok but then a page is missing a byte and the whole output gets offset.  I figured it out but I didn't see any way to recover.

The DMPAFT command works just fine on my unit but the data is garbage.  There are repeated dates, dates in the future (more than 1 day) and the data is completely out of order.

There are some hardcoded references to "Alameda"

I ignore and don't store a ton of the available data.
