package main

import (
	"flag"
	"fmt"
	"log"
	"strings"
	"time"

	"golang.org/x/net/context"

	"github.com/currantlabs/ble"
	"github.com/currantlabs/ble/examples/lib/dev"
)

var (
	device = flag.String("device", "default", "implementation of ble")
	sd     = flag.Duration("sd", 5*time.Second, "scanning duration, 0 for indefinitely")
)

const name = "Yudetamago config"
const command = "set_led 0 0 0 0\n"

func main() {
	flag.Parse()

	d, err := dev.NewDevice(*device)
	if err != nil {
		log.Fatalf("can't new device : %s", err)
	}
	ble.SetDefaultDevice(d)

	// Default to search device with name of Gopher (or specified by user).
	filter := func(a ble.Advertisement) bool {
		return strings.ToUpper(a.LocalName()) == strings.ToUpper(name)
	}

	// Scan for specified durantion, or until interrupted by user.
	fmt.Printf("Scanning for %s...\n", *sd)
	ctx := ble.WithSigHandler(context.WithTimeout(context.Background(), *sd))
	cln, err := ble.Connect(ctx, filter)
	if err != nil {
		log.Fatalf("can't connect : %s", err)
	}

	mtu, err := cln.ExchangeMTU(512)
	if err != nil {
		log.Fatalf("can't exchange MTU : %s", err)
	}
	log.Printf("exchange MTU : %d", mtu)

	// Make sure we had the chance to print out the message.
	done := make(chan struct{})
	// Normally, the connection is disconnected by us after our exploration.
	// However, it can be asynchronously disconnected by the remote peripheral.
	// So we wait(detect) the disconnection in the go routine.
	go func() {
		<-cln.Disconnected()
		fmt.Printf("[ %s ] is disconnected \n", cln.Address())
		close(done)
	}()

	fmt.Printf("Discovering profile...\n")
	p, err := cln.DiscoverProfile(true)
	if err != nil {
		log.Fatalf("can't discover profile: %s", err)
	}

	notifyCharacteristics, err := getNotifyCharacteristics(p)
	writeCharacteristics, err := getWriteCharacteristics(p)

	err = executeCommand(cln, command, writeCharacteristics, notifyCharacteristics)
	if err != nil {
		log.Fatalf("executeCommand() returns fail: %s", err)
	}

	// Disconnect the connection. (On OS X, this might take a while.)
	fmt.Printf("Disconnecting [ %s ]... (this might take up to few seconds on OS X)\n", cln.Address())
	cln.CancelConnection()

	<-done
}

func executeCommand(client ble.Client,
	command string,
	writeCharacteristics *ble.Characteristic,
	readCharacteristics *ble.Characteristic) error {
	client.WriteCharacteristic(writeCharacteristics, []byte(command), false)
	for {
		log.Print("[write] ", command)
		buf, err := client.ReadCharacteristic(readCharacteristics)
		if err != nil {
			log.Fatalf("ReadCharacteristic() returns fails: %s", err)
			return err
		}
		resp := string(buf)
		log.Print("[read ] ", resp)
		if strings.Contains(resp, "result") {
			break
		}
	}
	return nil
}

func getCharacteristicsImpl(profile *ble.Profile, property ble.Property) (*ble.Characteristic, error) {
	for _, s := range profile.Services {
		for _, c := range s.Characteristics {
			if (c.Property & property) != 0 {
				return c, nil
			}
		}
	}
	return nil, fmt.Errorf("not found %v Characteristic", property)
}

func getNotifyCharacteristics(p *ble.Profile) (*ble.Characteristic, error) {
	return getCharacteristicsImpl(p, ble.CharNotify)
}

func getWriteCharacteristics(p *ble.Profile) (*ble.Characteristic, error) {
	return getCharacteristicsImpl(p, ble.CharWrite)
}
