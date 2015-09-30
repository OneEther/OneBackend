#!/bin/bash

make
while true; do 
    ./echo -scanner -pool -web | tee output.log
    mv output.log log/$(date +"%Y%m%d_%H%M%S").log
    echo "DUMPED LOG"
    sleep 2
done
