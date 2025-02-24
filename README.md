# Brewing Temperature Monitor App

This app idea is to become a tool for monitoring and analyzing temperature conditions in spaces where fermentation vessels are placed.<br>
It enables users to log their data via the API that has been developed for this project.<br>

# Tech Stack:
- **golang** (gin framework)
- **influxDB**

## Currently Implemented Features/Functionalities

1. Forward device data to API
2. Get all data from all devices
3. Get data for a given device given it's device id
4. Get data for given location, e.g. at Brewing Room 1
5. Support data aggegation methods such as min,max,mean,sum (currently supported only at 3.)


## TODOs:

### API:
* support data aggregation also at get all data endpoint
* implement pagination 

### Features:
* find information about yeasts and their temperature profiles
* build a dashboard/ui
* display information on dashboard/ui about how temperature conditions affect the yeast
* provide suggestions about what the user should do regarding the tempeature control (e.g. raise, lower tempearture)


