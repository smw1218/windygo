# windygo
Reading weather readings from a Davis Vantage Vue using ethernet.  Built specifically for reporting wind at [Boardsports California](http://boardsportscalifornia.com/weather1/alameda-weather)
There's a bunch of stuff in here that's kinda specific to them right now.  Wind reports were my priority so not everything that the weather station does is implemented.

## Features

- Reads "LOOP" records that the station produces every ~2s and creates summaries every 1, 5 and 10 minutes
- Saves summaries to mysql
- Creates a png report using gnuplot and ImageMagick
- Runs on a raspberry pi

## Installation
Dependencies: 

     sudo apt-get install gnuplot imagemagick mysql-server

### Building
This program is written in go, but the version that runs on raspbian is too old.  Download version 1.4 or newer from
http://dave.cheney.net/unofficial-arm-tarballs.  You need to match the version to the raspberry pi you have.  

It's ok to just install go in the home directory.  Set you GOROOT and PATH to point to your new install:
  
     tar xzf  go1.4.2.linux-arm...
     export GOROOT=$(pwd)/go
     export PATH=$GOROOT/bin:$PATH
     mkdir -p gopath gopath/src gopath/bin gopath/pkg
     export GOPATH=$(pwd)/gopath
     go get github.com/smw1218/windygo
     go install github.com/smw1218/windygo

If everything went well, the binary should be in gopath/bin.

### Fonts

I use Roboto and RobotoCondensed (Google Fonts) for the gnuplots and ImageMagick.  These need to installed for them to work.  Luckily, they're install on Rasbian already.  ImageMagick requires an extra xml file arrows/fonts.xml which needs to be copied into /etc/ImageMagick:

     sudo cp arrows/fonts.xml /etc/ImageMagick
  
Edit /etc/ImageMagick/type.xml and add this line between `<typemap>` and `</typemap>`:

     <include file="fonts.xml" />

I also created a custom font using [fontcustom](http://fontcustom.com/). Ir provides all the compass directions.  The font is in arrows/CompassArrows/CompassArrows.ttf and must be copied to a system font directory (for some reason on raspberry pi, gnuplot did not work with just setting GDFONTPATH):

     sudo cp arrows/CompassArrows/CompassArrows.ttf /usr/share/fonts/truetype/freefont/

### Mysql
You have to create the database for windygo by hand, but it will do the rest. Launch mysql as root and create it:
     mysql -u root
     create database windygo;
     create user windygo;
     GRANT ALL ON windygo.* TO 'windygo'@'localhost';

I didn't implement using a password, if someone files an issue I'll do it.

### Running

     windygo -h <ip address of your vantage>:22222

## Bugs
I implemented the DMP and DMPAFT commands but both seem to have issues. The DMP returns several pages ok but then a page is missing a byte and the whole output gets offset.  I figured it out but I didn't see any way to recover.

The DMPAFT command works just fine by on my unit the data is garbage.  There are repeated dates, dates in the future (more than 1 day) and the data is completely out of order.

There are some hardcoded references to "Alameda"

I ignore and don't store a ton of the available data.
