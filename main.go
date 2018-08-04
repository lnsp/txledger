package main

import (
	"fmt"
	"os"
	"path"
	"syscall"

	"github.com/lnsp/txledger/ledger"
	"github.com/lnsp/txledger/ledger/account"
	"github.com/lnsp/txledger/ledger/account/container"
	"github.com/micro/cli"
	"golang.org/x/crypto/ssh/terminal"
)

const (
	flagForce      = "force"
	flagDatastore  = "data"
	flagAccount    = "account"
	flagChain      = "chain"
	flagComplexity = "complexity"

	fileAccount     = "accounts"
	fileLedger      = "ledger"
	categoryAccount = "Account"
	categoryChain   = "Blockchain"
)

func createAccount(c *cli.Context) {
	accountFolder := path.Join(c.GlobalString(flagDatastore), fileAccount)
	// Ensure folder exists
	_, err := os.Stat(accountFolder)
	if err != nil && os.IsNotExist(err) {
		err := os.MkdirAll(accountFolder, 0755)
		if err != nil {
			fmt.Fprintln(os.Stderr, "Could not create account folder")
			os.Exit(1)
		}
	}
	// Request keyphrase
	fmt.Fprint(os.Stdout, "Please enter a passphrase: ")
	passphrase, err := terminal.ReadPassword(int(syscall.Stdin))
	if err != nil {
		fmt.Fprintln(os.Stderr, "Could not read passphrase")
		os.Exit(1)
	}
	private := account.NewPrivate()
	cont, err := container.New(passphrase, private)
	if err != nil {
		fmt.Fprintln(os.Stderr, "Could not build container:", err)
		os.Exit(1)
	}
	// Store private key container
	accountPath := path.Join(accountFolder, private.String()+".json")
	if err := container.WriteToFile(cont, accountPath); err != nil {
		fmt.Fprintln(os.Stderr, "Could not write container:", err)
		os.Exit(1)
	}
	fmt.Fprintln(os.Stdout, "\nCreated account with address", private.String())
}

func showFunds(c *cli.Context) {

}

func transferFunds(c *cli.Context) {

}

func viewAccountHistory(c *cli.Context) {

}

func initializeChain(c *cli.Context) {
	datapath := c.GlobalString(flagDatastore)
	_, err := os.Stat(datapath)
	if err != nil && os.IsNotExist(err) {
		err := os.MkdirAll(datapath, 0755)
		if err != nil {
			fmt.Fprintln(os.Stderr, "Could not create datapath")
			os.Exit(1)
		}
	}
	ledgerPath := path.Join(datapath, fileLedger)
	_, err = os.Stat(ledgerPath)
	if err == nil && !c.Bool(flagForce) {
		fmt.Fprintf(os.Stderr, "Chain already exists, override with -%s flag\n", flagForce)
		os.Exit(1)
	}
	accountPath := path.Join(datapath, fileAccount, c.String(flagAccount)+".json")
	account, err := container.ReadFromFile(accountPath)
	if err != nil {
		fmt.Fprintln(os.Stderr, "Failed to read account container:", err)
		os.Exit(1)
	}
	fmt.Fprint(os.Stdout, "Please enter the passphrase: ")
	passphrase, err := terminal.ReadPassword(int(syscall.Stdin))
	if err != nil {
		fmt.Fprintln(os.Stderr, "Could not read passphrase")
		os.Exit(1)
	}
	fmt.Fprintln(os.Stdout)
	privateKey, err := account.Unlock(passphrase)
	if err != nil {
		fmt.Fprintln(os.Stderr, "Could not unlock account")
		os.Exit(1)
	}
	id, complexity := uint64(c.Int(flagChain)), uint64(c.Int(flagComplexity))
	fmt.Fprintf(os.Stdout, "Init chain with ID %d and start complexity %d\n", id, complexity)
	chain := ledger.New(id)
	chain.Init(complexity, privateKey)
	ledgerFile, err := os.OpenFile(ledgerPath, os.O_CREATE|os.O_RDWR, 0755)
	if err != nil {
		fmt.Fprintln(os.Stderr, "Could not open ledger file: ", err)
		os.Exit(1)
	}
	chain.WriteTo(ledgerFile)
	ledgerFile.Close()
}

func inspectBlocks(c *cli.Context) {

}

func mineBlocks(c *cli.Context) {

}

func verifyChain(c *cli.Context) {

}

func main() {
	app := cli.NewApp()
	app.HideVersion = true
	app.Name = "txledger"
	app.Usage = "Distributed cryptographic ledger"
	app.Flags = []cli.Flag{
		cli.StringFlag{
			Name:  flagDatastore,
			Usage: "path to chain storage",
			Value: os.Getenv("HOME") + "/.txledger/",
		},
	}
	app.Commands = []cli.Command{
		{
			Name:     "new",
			Category: categoryAccount,
			Usage:    "create a new account",
			Action:   createAccount,
		},
		{
			Name:     "funds",
			Category: categoryAccount,
			Usage:    "display funds associated with your accounts",
			Action:   showFunds,
		},
		{
			Name:     "transfer",
			Category: categoryAccount,
			Usage:    "transfer funds from your account",
			Action:   transferFunds,
		},
		{
			Name:     "book",
			Category: categoryAccount,
			Usage:    "view transaction history",
			Action:   viewAccountHistory,
		},
		{
			Name:     "init",
			Category: categoryChain,
			Usage:    "initialize a new blockchain",
			Action:   initializeChain,
			Flags: []cli.Flag{
				cli.StringFlag{
					Name:  flagAccount,
					Usage: "private account for coinbase TX",
				},
				cli.BoolFlag{
					Name:  flagForce,
					Usage: "override all existing data",
				},
				cli.IntFlag{
					Name:  flagChain,
					Usage: "unique chain identifier",
				},
				cli.IntFlag{
					Name:  flagComplexity,
					Usage: "starting complexity for genesis block",
				},
			},
		},
		{
			Name:     "inspect",
			Category: categoryChain,
			Usage:    "view chain state",
			Action:   inspectBlocks,
		},
		{
			Name:     "verify",
			Category: categoryChain,
			Usage:    "verify blockchain structure",
			Action:   verifyChain,
		},
		{
			Name:     "mine",
			Category: categoryChain,
			Usage:    "find new blocks and get rewarded",
			Action:   mineBlocks,
		},
	}
	app.Run(os.Args)
}
