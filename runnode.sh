#!/bin/bash

# some older test, doesn't work and complains and I get this message on command line: "QApplication::qAppName: Please instantiate the QApplication object first"
# I also can't enter text after command executes
#echo "Hello World!"
#exec konsole --noclose -e cat ~/.aliases

for i in 1 2 3 4 5 6 7 8 9 10 11 12 13 14 15 16
do
port = 8032 + i
ip = "localhost:"
addr = "${ip}${port}"
# opens terminal but then I can't control terminal afterwards
gnome-terminal -e "go run server/server.go -port ${addr}"  &
done

# didn't do anything
#exit 0

# didn't do anything except make me type exit an extra time where I executed my shell script
#$SHELL
