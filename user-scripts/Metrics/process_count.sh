#!/bin/bash
#This is a very simple user-script to collect number of running processes 

echo "Number_of_running_process" 

ps -aux | wc -l

