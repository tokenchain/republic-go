package registry

// const (
// 	reset  = "\x1b[0m"
// 	yellow = "\x1b[33;1m"
// 	green  = "\x1b[32;1m"
// 	red    = "\x1b[31;1m"
// )

// // Registry command-line tool for interacting with the darknodeRegister contract
// // on Kovan(default) or Ropsten testnet.
// func main() {

// 	// Load ethereum key
// 	key, err := LoadKey()
// 	if err != nil {
// 		log.Fatal("failt to load key from file", err)
// 	}

// 	// Create new cli application
// 	app := cli.NewApp()

// 	// Define flags
// 	app.Flags = []cli.Flag{
// 		cli.StringFlag{
// 			Name:  "network",
// 			Value: "kovan",
// 			Usage: "Ethereum testnet name",
// 		},
// 	}

// 	// Define subcommands
// 	app.Commands = []cli.Command{
// 		{
// 			Name:    "epoch",
// 			Aliases: []string{"e"},
// 			Usage:   "calling epoch",
// 			Action: func(c *cli.Context) error {
// 				registry, err := NewRegistry(c, key)
// 				if err != nil {
// 					return err
// 				}
// 				_, err = registryContract.TriggerEpoch()
// 				log.Println("Epoch called.")
// 				return err
// 			},
// 		},
// 		{
// 			Name:    "checkreg",
// 			Aliases: []string{"c"},
// 			Usage:   "check if the node is registered or not",
// 			Action: func(c *cli.Context) error {
// 				registrar, err := NewRegistry(c, key)
// 				if err != nil {
// 					return err
// 				}

// 				return CheckRegistration(c.Args(), registrar)
// 			},
// 		},
// 		{
// 			Name:    "register",
// 			Aliases: []string{"r"},
// 			Usage:   "register nodes in the dark node registry",
// 			Action: func(c *cli.Context) error {
// 				registry, err := NewRegistry(c, key)
// 				if err != nil {
// 					return err
// 				}
// 				return RegisterAll(registry)
// 			},
// 		},
// 		{
// 			Name:    "approve",
// 			Aliases: []string{"a"},
// 			Usage:   "approve nodes with enough REN token",
// 			Action: func(c *cli.Context) error {
// 				registry, err := NewRegistry(c, key)
// 				if err != nil {
// 					return err
// 				}
// 				return Approve(registry)
// 			},
// 		},
// 		{
// 			Name:    "deregister",
// 			Aliases: []string{"d"},
// 			Usage:   "deregister nodes in the dark node registry",
// 			Action: func(c *cli.Context) error {
// 				registry, err := NewRegistry(c, key)
// 				if err != nil {
// 					return err
// 				}
// 				return DeregisterAll(c.Args(), registry)
// 			},
// 		},
// 		{
// 			Name:  "refund",
// 			Usage: "refund ren",
// 			Action: func(c *cli.Context) error {
// 				registry, err := NewRegistry(c, key)
// 				if err != nil {
// 					return err
// 				}
// 				return Refund(c.Args(), registry)
// 			},
// 		},
// 		{
// 			Name:    "pool",
// 			Aliases: []string{"p"},
// 			Usage:   "get the index of the pool the node is in, return -1 if no pool found",
// 			Action: func(c *cli.Context) error {
// 				registrar, err := NewRegistry(c, key)
// 				if err != nil {
// 					return err
// 				}
// 				return GetPool(c.Args(), registrar)
// 			},
// 		},
// 	}
// 	err = app.Run(os.Args)
// 	if err != nil {
// 		log.Fatal(err)
// 	}
// }

// func LoadKey() (*keystore.Key, error) {
// 	var keyJSON string = `{"address":"90e6572ef66a11690b09dd594a18f36cf76055c8",
//   					"privatekey":"dc3f937b4aa1fc7bbf7643f1dead1faf37594ad2f1edcd6b56bf6719f85fa406",
//   					"id":"ddd54c1c-6c2e-42a9-a224-6532a90fd4e9", "version":3}`
// 	key := new(keystore.Key)
// 	err := key.UnmarshalJSON([]byte(keyJSON))

// 	return key, err
// }

// func NewRegistry(c *cli.Context, key *keystore.Key) (contract.Binder, error) {
// 	var config contract.Config
// 	switch c.GlobalString("network") {
// 	case "ropsten":
// 		config = contract.Config{
// 			Network:                 contract.NetworkRopsten,
// 			URI:                     "https://ropsten.infura.io",
// 			RepublicTokenAddress:    contract.RepublicTokenAddressOnRopsten.String(),
// 			DarknodeRegistryAddress: contract.DarknodeRegistryAddressOnRopsten.String(),
// 		}
// 	case "kovan":
// 		config = contract.Config{
// 			Network:                 contract.NetworkKovan,
// 			URI:                     "https://kovan.infura.io",
// 			RepublicTokenAddress:    contract.RepublicTokenAddressOnKovan.String(),
// 			DarknodeRegistryAddress: contract.DarknodeRegistryAddressOnKovan.String(),
// 		}
// 	default:
// 		log.Fatal("unrecognized network name")
// 	}

// 	auth := bind.NewKeyedTransactor(key.PrivateKey)
// 	auth.GasPrice = big.NewInt(5000000000)

// 	client, err := contract.Connect(config)
// 	if err != nil {
// 		log.Fatal("fail to connect to ethereum")
// 	}

// 	return contract.NewBinder(context.Background(), config, config)
// }

// func RegisterAll(registryContract contract.Binder) error {

// 	conf, err := loadConfig("./deployment.json")
// 	if err != nil {
// 		return errors.New("could not read file deployment.json")
// 	}

// 	for i := range conf.Configs {
// 		address := conf.Configs[i].Config.Address
// 		pk, err := crypto.BytesFromRsaPublicKey(&conf.Configs[i].Config.Keystore.RsaKey.PublicKey)
// 		if err != nil {
// 			log.Fatal("cannot get rsa public key ", err)
// 		}

// 		// Check if node has already been registered
// 		isRegistered, err := registryContract.IsRegistered(address)
// 		if err != nil {
// 			return fmt.Errorf("[%v] %sCouldn't check node's registration%s: %v\n", address, red, reset, err)
// 		}

// 		// Register the node if not registered
// 		if !isRegistered {
// 			minimumBond, err := registryContract.MinimumBond()
// 			if err != nil {
// 				return err
// 			}

// 			_, err = registryContract.Register(address.ID(), pk, &minimumBond)
// 			if err != nil {
// 				return fmt.Errorf("[%v] %sCouldn't register node%s: %v\n", address, red, reset, err)
// 			} else {
// 				log.Printf("[%v] %sNode will be registered next epoch%s\n", address, green, reset)
// 			}
// 		} else if isRegistered {
// 			log.Printf("[%v] %sNode already registered%s\n", address, yellow, reset)
// 		}
// 	}

// 	return nil

// }

// // DeregisterAll takes a slice of republic private keys and registers them
// func DeregisterAll(addresses []string, registryContract contract.Binder) error {
// 	conf, err := loadConfig("./deployment.json")
// 	if err != nil {
// 		return errors.New("could not read file deployment.json")
// 	}

// 	for i := range conf.Configs {
// 		address := conf.Configs[i].Config.Address

// 		// Check if node has already been registered
// 		isRegistered, err := registryContract.IsRegistered(address)
// 		if err != nil {
// 			return fmt.Errorf("[%v] %sCouldn't check node's registration%s: %v\n", address, red, reset, err)
// 		}

// 		if isRegistered {
// 			_, err = registryContract.Deregister(address.ID())
// 			if err != nil {
// 				return fmt.Errorf("[%v] %sCouldn't deregister node%s: %v\n", address, red, reset, err)
// 			} else {
// 				log.Printf("[%v] %sNode will be deregistered next epoch%s\n", address, green, reset)
// 			}
// 		} else {
// 			log.Printf("[%v] %sNode already registered%s\n", address, yellow, reset)
// 		}
// 	}

// 	return nil
// }

// func Approve(registryContract contract.Binder) error {

// 	bond, err := stackint.FromString("100000000000000000000000")
// 	if err != nil {
// 		return err
// 	}
// 	_, err = registryContract.ApproveRen(&bond)
// 	if err != nil {
// 		return err
// 	}

// 	return nil
// }

// // GetPool will get the index of the pool the node is in.
// // The address should be the ethereum address
// func GetPool(addresses []string, registryContract contract.Binder) error {
// 	if len(addresses) != 1 {
// 		return fmt.Errorf("%sPlease provide one node address.%s\n", red, reset)
// 	}

// 	pod, err := registryContract.Pod(identity.Address(addresses[0]))
// 	if err != nil {
// 		fmt.Println(-1)
// 		return err
// 	}
// 	fmt.Println(pod.Position)

// 	return nil
// }

// // CheckRegistration will check if the node with given address is registered with
// // the darknode registryContract. The address will be the ethereum address.
// func CheckRegistration(addresses []string, contract contract.Binder) error {
// 	if len(addresses) != 1 {
// 		return fmt.Errorf("%sPlease provide one node address.%s\n", red, reset)
// 	}

// 	isRegistered, err := contract.IsRegistered(identity.Address(addresses[0]))
// 	if err != nil {
// 		return err
// 	}
// 	fmt.Println(isRegistered)

// 	return nil
// }

// func Refund(addresses []string, registryContract contract.Binder) error {
// 	for i := range addresses {
// 		address, err := republicAddressToEthAddress(addresses[i])
// 		if err != nil {
// 			return err
// 		}
// 		_, err = registryContract.Refund(address.Bytes())
// 		if err != nil {
// 			return err
// 		}
// 		log.Printf("[%v] %sNode has been refunded%s\n", address.Hex(), green, reset)
// 	}

// 	return nil
// }

// // Convert republic address to ethereum address
// func republicAddressToEthAddress(repAddress string) (common.Address, error) {
// 	addByte := base58.DecodeAlphabet(repAddress, base58.BTCAlphabet)[2:]
// 	if len(addByte) == 0 {
// 		return common.Address{}, errors.New("fail to decode the address")
// 	}
// 	address := common.BytesToAddress(addByte)
// 	return address, nil
// }

// func loadConfig(filename string) (FalconryConfigs, error) {
// 	file, err := os.Open(filename)
// 	if err != nil {
// 		return FalconryConfigs{}, err
// 	}
// 	defer file.Close()

// 	data, err := ioutil.ReadAll(file)

// 	conf := FalconryConfigs{}
// 	if err := json.Unmarshal(data, &conf); err != nil {
// 		return FalconryConfigs{}, err
// 	}

// 	return conf, nil
// }

// type FalconryConfigs struct {
// 	Configs    []FalconryConfig `json:"configs"`
// 	Blockchain interface{}      `json:"blockchain"`
// }

// type FalconryConfig struct {
// 	Config      config.Config `json:"config"`
// 	Ami         string        `json:"ami"`
// 	Avz         string        `json:"avz"`
// 	Instance    string        `json:"instance"`
// 	Ip          string        `json:"ip"`
// 	IsBootstrap bool          `json:"is_bootstrap"`
// }
