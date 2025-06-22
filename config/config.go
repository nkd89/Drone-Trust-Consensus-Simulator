package config

type SimulatorConfig struct {
	AlgorithmName        string
	ResultsDir           string
	NumDrones            int
	MaliciousRatio       float64
	AreaWidth            float64
	AreaHeight           float64
	SimulationTime       float64
	CHReelectionInterval float64
	PacketGenInterval    float64

	// Параметры выбора CH
	CHSelectionAlgorithm string // "BaseBTMSD", "PoRS", "Blockchain"

	// Параметры доверия
	AlphaTrust     float64 // Фактор забывания для простого доверия
	TrustThreshold float64
	LambdaDecay    float64 // λ из формулы (2) статьи для затухания ист. доверия

	// Параметры узлов
	InitialEnergy       float64
	EnergyMin           float64
	EnergyTx            float64 // Энергия на передачу пакета
	EnergyRx            float64 // Энергия на прием
	EnergyConsensus     float64 // Энергия на раунд консенсуса
	MinCompPower        float64 // Минимальная выч. мощность (Gflops)
	MaxCompPower        float64 // Максимальная выч. мощность (Gflops)
	CommunicationRadius float64

	// Параметры консенсуса (для 'Blockchain')
	ConsensusType   string  // "PBFT", "PoW"
	PoWMiningTime   float64 // Среднее время майнинга блока PoW
	PBFTBaseLatency float64 // Базовая задержка на раунд PBFT

	// Параметры доверия
	TrustModel        string  // "Simple", "Complex", "TrustByDefault"
	InitialTrustValue float64 // Начальное доверие к узлам
}

const (
	numDrones           = 100
	simulationTime      = 120.0
	areaWidth           = 800.0
	areaHeight          = 800.0
	communicationRadius = 250.0
)

func GetBaseBTMSDConfig() *SimulatorConfig {
	return &SimulatorConfig{
		AlgorithmName:        "Base BTMSD",
		ResultsDir:           "BaseBTMSD",
		NumDrones:            numDrones,
		MaliciousRatio:       0.2,
		AreaWidth:            areaWidth,
		AreaHeight:           areaHeight,
		SimulationTime:       simulationTime,
		CHReelectionInterval: 10.0,
		PacketGenInterval:    1.0,
		CHSelectionAlgorithm: "BaseBTMSD",
		AlphaTrust:           0.3,
		TrustThreshold:       0.5,
		LambdaDecay:          0.1,
		InitialEnergy:        5000.0,
		EnergyMin:            500.0,
		EnergyTx:             0.5,
		EnergyRx:             0.1,
		EnergyConsensus:      0,
		MinCompPower:         1.0,
		MaxCompPower:         2.0,
		TrustModel:           "Simple",
		InitialTrustValue:    0.5,
		CommunicationRadius:  communicationRadius,
	}
}

func GetPoRSConfig() *SimulatorConfig {
	cfg := GetBaseBTMSDConfig()
	cfg.AlgorithmName = "Proof-of-Reputation-and-Services (PoRS)"
	cfg.ResultsDir = "PoRS"
	cfg.CHSelectionAlgorithm = "PoRS"
	return cfg
}

func GetBlockchainConfig() *SimulatorConfig {
	cfg := GetBaseBTMSDConfig()
	cfg.AlgorithmName = "Hierarchical Blockchain (PBFT)"
	cfg.ResultsDir = "Blockchain_PBFT"
	cfg.CHSelectionAlgorithm = "Blockchain" // Лидер выбирается по RF, FF
	cfg.ConsensusType = "PBFT"
	cfg.PBFTBaseLatency = 0.5
	cfg.PoWMiningTime = 5.0
	cfg.EnergyConsensus = 2.0
	cfg.TrustModel = "Complex"
	return cfg
}

func GetPoRSBlockchainConfig() *SimulatorConfig {
	cfg := GetBlockchainConfig() // Берем за основу блокчейн-конфиг
	cfg.AlgorithmName = "PoRS + Blockchain (PoW)"
	cfg.ResultsDir = "PoRS_Blockchain_PoW"
	// Лидер по-прежнему выбирается по RF и FF, так как это лидер КОНСЕНСУСА
	// А вот консенсус меняем на PoW
	cfg.ConsensusType = "PoW"
	// Выбор CH будет использовать PoRS, а не Blockchain-метрики
	// Но у нас CH и лидер консенсуса - одно и то же. Давайте это разделим.
	// Для чистоты эксперимента, пусть в этой модели для выбора CH (лидера) используется PoRS, а не RF+FF.
	cfg.CHSelectionAlgorithm = "PoRS"
	cfg.TrustModel = "Complex"
	return cfg
}

func GetPoRSConsensusConfig() *SimulatorConfig {
	cfg := GetBaseBTMSDConfig() // Берем за основу
	cfg.AlgorithmName = "Reputation-Based Consensus (PoRS)"
	cfg.ResultsDir = "PoRS_Consensus"
	// CH выбирается по правилам PoRS
	cfg.CHSelectionAlgorithm = "PoRS"
	// Тип консенсуса тоже PoRS
	cfg.ConsensusType = "PoRS_Consensus"
	// Включаем накладные расходы на энергию для консенсуса
	cfg.EnergyConsensus = 1.0 // Меньше, чем у PBFT, но больше, чем у PoW
	// Включаем использование блокчейна для модели доверия
	cfg.TrustModel = "Complex"
	cfg.CHSelectionAlgorithm = "Blockchain" // Это заставит использовать комплексную модель доверия из статьи
	// НО! Лидер консенсуса (и CH) все равно будет выбираться по PoRS. Это тонкий момент.
	// Давайте сделаем так:
	// 1. Метод консенсуса - наш новый PoRS_Consensus
	// 2. Метод выбора CH - тоже PoRS.
	// 3. Модель доверия - комплексная, из статьи (как для блокчейна)
	// Это создаст самый интересный гибрид.

	finalCfg := GetPoRSConfig() // Начнем с PoRS конфига
	finalCfg.AlgorithmName = "Reputation-Based Consensus (PoRS)"
	finalCfg.ResultsDir = "PoRS_Consensus"
	finalCfg.ConsensusType = "PoRS_Consensus"
	finalCfg.EnergyConsensus = 1.0

	// Заставим использовать комплексную модель доверия.
	// Для этого в RecordInteraction мы должны смотреть не на CHSelectionAlgorithm, а на ConsensusType.
	// Давайте упростим: если есть консенсус, то модель доверия - комплексная.
	// Это изменение нужно внести в trust/manager.go
	return finalCfg
}

func GetUnifiedPoRSConfig() *SimulatorConfig {
	cfg := GetPoRSConfig()
	cfg.AlgorithmName = "Behavior-Aware Reputation Consensus (BARC)" // Новое имя
	cfg.ResultsDir = "BARC"

	cfg.CHSelectionAlgorithm = "BARC"
	cfg.ConsensusType = "PoRS_Consensus"
	cfg.EnergyConsensus = 1.0

	// <<< ГЛАВНЫЕ ИЗМЕНЕНИЯ >>>
	cfg.TrustModel = "TrustByDefault"
	cfg.InitialTrustValue = 0.9 // Высокое начальное доверие

	return cfg
}
