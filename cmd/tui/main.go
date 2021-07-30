package main

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log"
	"mapdownloader/config"
	"mapdownloader/internal/downloader"
	"reflect"
	"strconv"
	"time"

	"code.cloudfoundry.org/bytefmt"
	"github.com/charmbracelet/bubbles/progress"
	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

func main() {
	p := tea.NewProgram(new(model))
	if err := p.Start(); err != nil {
		log.Fatal(err)
	}
}

type model struct {
	err           string
	state         int
	tilesErr      int
	tilesDone     int
	tilesCount    int
	percent       float64
	spinner       spinner.Model
	progress      *progress.Model
	textInput     textinput.Model
	downloader    *downloader.DownLoader
	helpTextStyle func(str string) string
}

func (m *model) Init() tea.Cmd {
	ti := textinput.NewModel()
	ti.Focus()
	ti.Width = 180
	ti.Placeholder = "eyJ0eXBlIjoiMCIsIm..."

	s := spinner.NewModel()
	s.Spinner = spinner.Dot
	s.Style = lipgloss.NewStyle().Foreground(lipgloss.Color("205"))

	m.state = 1
	m.percent = 0.0
	m.spinner = s
	m.textInput = ti
	m.progress, _ = progress.NewModel(progress.WithScaledGradient("#FF7CCB", "#FDFF8C"))
	m.helpTextStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#626262")).Render

	//return tickCmd()
	return spinner.Tick
}

func (m *model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	if reflect.TypeOf(msg).String() == "tea.KeyMsg" {
		mg := msg.(tea.KeyMsg)
		if mg.Type == tea.KeyCtrlC || mg.Type == tea.KeyEsc {
			return m, tea.Quit
		} else if mg.Type == tea.KeyEnter {
			switch m.state {
			case 1:
				m.state = 1
				m.preDownload()
			case 2:
				m.state = 1
			case 3:
				m.state = 4
				go m.runDownload()
			case 5:
				return m, tea.Quit
			default:
				return m, nil
			}
			return m, nil
		} else {
			text, cmd := m.textInput.Update(msg)
			m.textInput = text
			return m, cmd
		}
	}
	m.spinner, cmd = m.spinner.Update(msg)
	return m, cmd
}

func (m *model) preDownload() {
	if mi, err := base64.StdEncoding.DecodeString(m.textInput.Value()); err != nil {
		m.err = "Configuration analysis failed ( base64 )"
		m.state = 2
		return
	} else {
		mapInfo := downloader.MapInfo{}
		if err := json.Unmarshal(mi, &mapInfo); err != nil {
			m.err = "Configuration analysis failed ( json unmarshal )"
			m.state = 2
			return
		} else {
			mapInfo.DbPath = "./mapTiles.db"
			m.downloader = downloader.NewDownLoader(mapInfo, 4096, 4096, 512)
			m.tilesCount = m.downloader.GetTaskInfo()
			m.state = 3
		}
	}
}

func (m *model) runDownload() {
	go m.downloader.Start()
	for {
		time.Sleep(time.Millisecond * 500)
		m.percent, m.tilesDone, m.tilesErr = m.downloader.GetDownPercent()
		if m.percent >= float64(1) {
			m.state = 5
			return
		}
	}
}

func (m *model) View() (str string) {
	str = "MapDownloader " + config.VERSION + "\n\n"
	switch m.state {
	case 1:
		str += "Enter the configuration string from the website <map.lizhengtech.com> \n\n" +
			m.textInput.View() + "\n\n"
	case 2:
		str += "err: " + m.err + "\n\n"
	case 3:
		str += " Guage Size: " + bytefmt.ByteSize(uint64(m.tilesCount*1024*15)) + "\n" +
			"Tiles Count: " + strconv.Itoa(m.tilesCount) + "\n\n" +
			"Press Enter Start Download ..." + "\n\n"
	case 4:
		str += m.spinner.View() + " Processing...   " +
			fmt.Sprintf("total: %d   done: %d  timeout: %d", m.tilesCount, m.tilesDone, m.tilesErr) +
			"\n\n" + m.progress.View(m.percent) + "\n\n"
	case 5:
		str += "download successful \n\n upload : oss cp /root/mapTools/mapTiles.db oss://lz-map/ \n\n   url: https://lz-map.oss-cn-shenzhen.aliyuncs.com/mapTiles.db \n\n"
	default:
		return fmt.Sprintf("err")
	}
	str += m.helpTextStyle("(esc to quit)") + "\n"
	return
}

func tickCmd() tea.Cmd {
	return tea.Tick(time.Millisecond*200, func(t time.Time) tea.Msg {
		return time.Time(t)
	})
}
