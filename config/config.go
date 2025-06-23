// Файл: config/config.go
package config

import "fmt"

// --- Структура конфига остается прежней ---
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
	CHSelectionAlgorithm string
	TrustModel           string
	AlphaTrust           float64
	TrustThreshold       float64
	InitialTrustValue    float64
	LambdaDecay          float64
	InitialEnergy        float64
	EnergyMin            float64
	EnergyTx             float64
	EnergyRx             float64
	EnergyConsensus      float64
	MinCompPower         float64
	MaxCompPower         float64
	CommunicationRadius  float64
	ConsensusType        string
	PoWMiningTime        float64
	PBFTBaseLatency      float64
}

// --- Базовый шаблон со значениями по умолчанию ---
func getBaseTemplate() *SimulatorConfig {
	return &SimulatorConfig{
		SimulationTime:       120.0,
		CHReelectionInterval: 10.0,
		PacketGenInterval:    1.0,
		AlphaTrust:           0.3,
		TrustThreshold:       0.5,
		InitialTrustValue:    0.5,
		LambdaDecay:          0.1,
		InitialEnergy:        5000.0,
		EnergyMin:            500.0,
		EnergyTx:             0.5,
		EnergyRx:             0.1,
		MinCompPower:         1.0,
		MaxCompPower:         2.0,
		PoWMiningTime:        5.0,
		PBFTBaseLatency:      0.5,
	}
}

// --- Шаблоны для каждого из 6 алгоритмов ---

func getBTMSDTemplate() *SimulatorConfig {
	cfg := getBaseTemplate()
	cfg.AlgorithmName = "Base BTMSD"
	cfg.CHSelectionAlgorithm = "BaseBTMSD"
	cfg.TrustModel = "Simple"
	cfg.ConsensusType = ""
	return cfg
}

func getPoRSTemplate() *SimulatorConfig {
	cfg := getBaseTemplate()
	cfg.AlgorithmName = "PoRS"
	cfg.CHSelectionAlgorithm = "PoRS"
	cfg.TrustModel = "Simple"
	cfg.ConsensusType = ""
	return cfg
}

func getPBFTTemplate() *SimulatorConfig {
	cfg := getBaseTemplate()
	cfg.AlgorithmName = "Blockchain (PBFT)"
	cfg.CHSelectionAlgorithm = "Blockchain"
	cfg.TrustModel = "Complex"
	cfg.ConsensusType = "PBFT"
	cfg.EnergyConsensus = 2.0
	return cfg
}

func getPoWTemplate() *SimulatorConfig {
	cfg := getBaseTemplate()
	cfg.AlgorithmName = "PoRS + Blockchain (PoW)"
	cfg.CHSelectionAlgorithm = "PoRS"
	cfg.TrustModel = "Complex"
	cfg.ConsensusType = "PoW"
	cfg.EnergyConsensus = 1.5
	return cfg
}

func getReputationConsensusTemplate() *SimulatorConfig {
	cfg := getBaseTemplate()
	cfg.AlgorithmName = "Reputation-Based Consensus"
	cfg.CHSelectionAlgorithm = "PoRS"
	cfg.TrustModel = "Simple"
	cfg.ConsensusType = "PoRS_Consensus"
	cfg.EnergyConsensus = 1.0
	return cfg
}

func getUnifiedPORSTemplate() *SimulatorConfig {
	cfg := getBaseTemplate()
	cfg.AlgorithmName = "BARC"
	cfg.CHSelectionAlgorithm = "Unified PoRS Consensus"
	cfg.TrustModel = "TrustByDefault"
	cfg.InitialTrustValue = 0.9
	cfg.ConsensusType = "PoRS_Consensus"
	cfg.EnergyConsensus = 1.0
	return cfg
}

// <<< ГЛАВНАЯ ФУНКЦИЯ-ГЕНЕРАТОР >>>
// GenerateExperimentConfigs создает список всех конфигураций для полного факторного эксперимента.
func GenerateExperimentConfigs() []*SimulatorConfig {
	// --- Определяем шаблоны для каждого алгоритма ---
	templates := []*SimulatorConfig{
		getBTMSDTemplate(),
		getPoRSTemplate(),
		getPBFTTemplate(),
		// getPoWTemplate(),
		// getReputationConsensusTemplate(),
		getUnifiedPORSTemplate(),
	}

	// --- Определяем диапазоны параметров, которые мы будем варьировать ---
	numDronesRange := []int{50, 100}
	maliciousRatioRange := []float64{0.1, 0.3, 0.7}
	areaSizeRange := []float64{200.0, 400.0, 800.0, 1200.0}

	var allConfigs []*SimulatorConfig

	// --- Создаем комбинации вложенными циклами ---
	for _, tpl := range templates {
		for _, numDrones := range numDronesRange {
			for _, maliciousRatio := range maliciousRatioRange {
				for _, areaSize := range areaSizeRange {

					// Создаем копию шаблона
					cfg := *tpl

					// Применяем варьируемые параметры
					cfg.NumDrones = numDrones
					cfg.MaliciousRatio = maliciousRatio
					cfg.AreaWidth = areaSize
					cfg.AreaHeight = areaSize

					// Радиус связи можно сделать зависимым от плотности
					// Простое правило: 1/4 от размера площади
					cfg.CommunicationRadius = areaSize / 4.0

					// Формируем уникальное имя директории для результатов
					cfg.ResultsDir = fmt.Sprintf("%s/drones_%d_malicious_%.1f_area_%.0f",
						tpl.AlgorithmName,
						numDrones,
						maliciousRatio,
						areaSize)

					// Добавляем готовую конфигурацию в общий список
					allConfigs = append(allConfigs, &cfg)
				}
			}
		}
	}
	return allConfigs
}
