package main

import (
	"flag"
	"fmt"
	"log"
	"math"
	"os"
	"time"

	"github.com/robvanmieghem/go-opencl/cl"
)

var intensity = 22
var globalItemSize int
var devicesTypesForMining = cl.DeviceTypeGPU

func createWork(miningWorkChannel chan *MiningWork, nrOfWorkItemsPerRequestedHeader int) {
	for {
		target, header, err := getHeaderForWork()
		if err != nil {
			log.Println("ERROR fetching work -", err)
			time.Sleep(1)
			continue
		}
		//copy target to header
		for i := 0; i < 8; i++ {
			header[i+32] = target[7-i]
		}

		for i := 0; i < nrOfWorkItemsPerRequestedHeader; i++ {
			miningWorkChannel <- &MiningWork{header, i * globalItemSize}
		}
	}
}

func main() {
	useCPU := flag.Bool("cpu", false, "If set, also use the CPU for mining, only GPU's are used by default")
	flag.IntVar(&intensity, "I", intensity, "Intensity")

	flag.Parse()

	if *useCPU {
		devicesTypesForMining = cl.DeviceTypeAll
	}
	globalItemSize = int(math.Exp2(float64(intensity)))

	platforms, err := cl.GetPlatforms()
	if err != nil {
		log.Panic(err)
	}

	clDevices := make([]*cl.Device, 0, 4)
	for _, platform := range platforms {
		log.Println("Platform", platform.Name())
		platormDevices, err := cl.GetDevices(platform, devicesTypesForMining)
		if err != nil {
			log.Println(err)
		}
		log.Println(len(platormDevices), "device(s) found:")
		for i, device := range platormDevices {
			log.Println(i, "-", device.Type(), "-", device.Name())
			clDevices = append(clDevices, device)
		}
	}

	nrOfMiningDevices := len(clDevices)

	if nrOfMiningDevices == 0 {
		log.Println("No suitable opencl devices found")
		os.Exit(1)
	}

	//Fetch work
	workChannel := make(chan *MiningWork, nrOfMiningDevices*4)
	go createWork(workChannel, nrOfMiningDevices*2)

	//Start mining routines
	var hashRateReportsChannel = make(chan *HashRateReport, nrOfMiningDevices*10)
	for i, device := range clDevices {
		go mine(device, i, hashRateReportsChannel, workChannel)
	}

	hashRateReports := make([]float64, nrOfMiningDevices)
	for {
		report := <-hashRateReportsChannel
		hashRateReports[report.MinerID] = report.HashRate
		fmt.Print("\r")
		var totalHashRate float64
		for minerID, hashrate := range hashRateReports {
			fmt.Printf("%d - Mining at %.3f MH/s | ", minerID, hashrate)
			totalHashRate += hashrate
		}
		fmt.Printf("Total: %.3f MH/s", totalHashRate)
	}
}
