package main

import (
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"regexp"
	"strings"
)

//
const (
	defaultPartitionName = "nonePartition"
	spaceSep             = " "
)

//
type F5Config struct {
	File       string
	Partitions []*Partition
}

//
func NewF5Config(file string) *F5Config {
	return &F5Config{File: file}
}

//
func (p *F5Config) Parse() error {
	var err error
	content, err := ioutil.ReadFile(p.File)
	if err != nil {
		return err
	}

	contents := strings.Split(string(content), "\n")

	p.Partitions = splitPartitionLines(contents)

	for _, p := range p.Partitions {
		err = p.Parse()
		if err != nil {
			log.Printf("%s\n", err)
		}
	}

	return nil

}

//
func (p *F5Config) WritePools(file string) {
	var out []string = make([]string, 0)
	out = append(out, fmt.Sprintf("%s,%s,%s,%s,%s", "partition", "poolName", "lb_method", "member_ip", "member_port"))
	for _, v := range p.Partitions {
		for _, vv := range v.Pools {
			var mems = make([]string, 0)
			for _, vvv := range vv.Members {
				mems = append(mems, fmt.Sprintf("%s,%s", vvv.IP, vvv.Port))
			}

			for _, m := range mems {
				out = append(out, fmt.Sprintf("%s,%s,%s,%s", v.Name, vv.Name, vv.LBMethod, m))
			}
		}
	}

	var data = make([]byte, 0)
	for _, v := range out {
		s := []byte(v)
		s = append(s, []byte("\n")...)
		data = append(data, s...)
	}
	err := ioutil.WriteFile(file, data, os.ModePerm)
	if err != nil {
		fmt.Printf("[INFO] %s\n", err)
	}

}

//
func (p *F5Config) WriteVS(file string) {
	var out = make([]string, 0)
	out = append(out, fmt.Sprintf("%s,%s,%s,%s,%s,%s,%s", "partition", "vsname", "destination",
		"pool or rules", "protocol", "snat", "status"))
	for _, v := range p.Partitions {
		for _, vv := range v.VirtualServers {
			out = append(out, fmt.Sprintf("%s,%s", v.Name, vv))
		}
	}
	var data = make([]byte, 0)
	for _, v := range out {
		s := []byte(v)
		s = append(s, []byte("\n")...)
		data = append(data, s...)
	}
	err := ioutil.WriteFile(file, data, os.ModePerm)
	if err != nil {
		fmt.Printf("[INFO] %s\n", err)
	}
}

// split all contents with partitions contents
func splitPartitionLines(contents []string) []*Partition {
	match := regexp.MustCompile("^shell\\swrite\\spartition\\s.*")

	partitions := make([]*Partition, 0)

	for _, v := range contents {
		r := match.FindString(v)
		if len(r) != 0 {
			array := strings.Split(v, spaceSep)
			if len(array) != 4 {
				continue
			}
			sublines, err := getPartitionLines(array[3], contents)
			if err != nil {
				log.Printf("[INFO] get partition %s lines failed. error: %s\n", array[3], err)
				continue
			}

			partitions = append(partitions, &Partition{Name: array[3], Contents: sublines})
		}
	}

	if len(partitions) == 0 {
		partitions = append(partitions, &Partition{Name: defaultPartitionName, Contents: contents})
	}

	return partitions
}

// get partition lines by giving partition name
func getPartitionLines(name string, contents []string) (lines []string, err error) {
	match1 := regexp.MustCompile(fmt.Sprintf("^shell\\swrite\\spartition\\s%s", name))
	match2 := regexp.MustCompile("^shell\\swrite\\spartition\\s")
	var (
		start int = -1
		end   int = -1
		idx   int
	)
	for i, v := range contents {
		if start == -1 {
			r := match1.FindString(v)
			if len(r) != 0 {
				start = i
				continue
			}
		} else {
			r := match2.FindString(v)
			if len(r) != 0 {
				end = i
				break
			}
		}
		idx = i
	}
	if end == -1 && idx == len(contents)-1 {
		end = idx
	}

	if start >= end {
		err = fmt.Errorf("[INFO] start %d large than or equl end %d", start, end)
		return
	}
	return contents[start+1 : end], nil
}

//
type Partition struct {
	Name           string
	Contents       []string
	Pools          []*Pool
	VirtualServers []*VirtualServer
}

//
func (p *Partition) Parse() error {
	matchPool := regexp.MustCompile("^pool\\s.*\\s\\{")
	matchVS := regexp.MustCompile("^virtual\\s.*\\s\\{")
	matchVA := regexp.MustCompile("^virtual\\saddress\\s.*\\s\\{")

	p.Pools = make([]*Pool, 0)
	p.VirtualServers = make([]*VirtualServer, 0)

	for _, v := range p.Contents {
		r := matchPool.FindString(v)
		if len(r) != 0 {
			array := strings.Split(r, spaceSep)
			if len(array) != 3 {
				fmt.Printf("[INFO] get pool name failed. content: %s\n", r)
				continue
			}
			contents, err := getPoolLines(array[1], p.Contents)
			if err != nil {
				fmt.Printf("[INFO] get pool %s lines failed, error: %s\n", array[1], err)
				continue
			}
			p.Pools = append(p.Pools, &Pool{Name: array[1], Contents: contents})
			continue
		}

		r = matchVS.FindString(v)
		if len(r) != 0 {
			rr := matchVA.FindString(r)
			if len(rr) != 0 {
				continue
			}
			array := strings.Split(r, spaceSep)
			if len(array) != 3 {
				fmt.Printf("[INFO] get virtual server name failed. content: %s\n", r)
				continue
			}
			contents, err := getVSLines(array[1], p.Contents)
			if err != nil {
				fmt.Printf("[INFO] get virtual server %s lines failed, error: %s\n", array[1], err)
				continue
			}
			p.VirtualServers = append(p.VirtualServers, &VirtualServer{Name: array[1], Contents: contents})
		}
	}

	for _, v := range p.Pools {
		v.Parse()
	}

	for _, v := range p.VirtualServers {
		v.Parse()
	}

	return nil
}

//
func getPoolLines(name string, contents []string) (lines []string, err error) {
	match1 := regexp.MustCompile(fmt.Sprintf("^pool\\s%s\\s\\{", name))
	match2 := regexp.MustCompile("^\\}")
	var (
		start int = -1
		end   int = -1
	)
	for i, v := range contents {
		if start == -1 {
			r := match1.FindString(v)
			if len(r) != 0 {
				start = i
				continue
			}
		} else {
			r := match2.FindString(v)
			if len(r) != 0 {
				end = i
				break
			}
		}
	}

	if start > end {
		err = fmt.Errorf("[INFO] start(%d) large than or equl end(%d)", start, end)
		return
	}
	return contents[start : end+1], nil
}

//
//
func getVSLines(name string, contents []string) (lines []string, err error) {
	match1 := regexp.MustCompile(fmt.Sprintf("^virtual\\s%s\\s\\{", name))
	match2 := regexp.MustCompile("^\\}")
	var (
		start int = -1
		end   int = -1
	)
	for i, v := range contents {
		if start == -1 {
			r := match1.FindString(v)
			if len(r) != 0 {
				start = i
				continue
			}
		} else {
			r := match2.FindString(v)
			if len(r) != 0 {
				end = i
				break
			}
		}
	}
	if start >= end {
		err = fmt.Errorf("[INFO] start(%d) large than or equl end(%d)", start, end)
		return
	}
	return contents[start : end+1], nil
}

//
type Pool struct {
	Name     string
	Contents []string
	LBMethod string
	Monitors []*Monitor
	Members  []*Member
}

//
type Member struct {
	IP   string
	Port string
}

//
type Monitor struct {
	Object string
	Method string
}

// monitor all esb_monitor and tcp
func parseMonitors(line string) (monitors []*Monitor, err error) {
	array := strings.Split(strings.TrimSpace(line), spaceSep)
	if len(array) < 3 {
		err = fmt.Errorf("[INFO] monitor field wrong, %s\n", line)
		return
	}
	object := array[1]
	monitors = make([]*Monitor, 0)
	if strings.Contains(line, "and") {
		array1 := strings.Split(line, fmt.Sprintf("monitor %s", object))
		if len(array1) != 2 {
			err = fmt.Errorf("monitor field wrong")
			return
		}
		methods := strings.Split(array1[1], "and")
		for _, v := range methods {
			monitors = append(monitors, &Monitor{Object: object, Method: v})
		}
		return
	}

	monitors = append(monitors, &Monitor{Object: object, Method: array[2]})
	return
}

//
func (p *Pool) Parse() {
	//var err error
	//p.Monitors = make([]*Monitor, 0)
	p.Members = make([]*Member, 0)
	for _, v := range p.Contents {
		//		if strings.Contains(v, "monitor") {
		//			p.Monitors, err = parseMonitors(v)
		//			if err != nil {
		//				fmt.Printf("parse pool %s monitor failed. error: %s\n", p.Name, err)
		//			}
		//			continue
		//		}

		if strings.Contains(v, "lb method") {
			array := strings.Split(strings.TrimSpace(v), "method")
			if len(array) != 2 {
				fmt.Printf("[INFO] pool %s 's lb method: %s\n", p.Name, v)
				continue
			}
			p.LBMethod = array[1]
			continue
		}

		if strings.Contains(v, ":") {
			array := strings.Split(v, ":")
			if len(array) != 2 {
				fmt.Printf("[INFO] pool %s 's member: %s\n", p.Name, v)
				continue
			}
			array1 := strings.Split(strings.TrimSpace(array[0]), spaceSep)
			array2 := strings.Split(array[1], spaceSep)
			if len(array2) != 2 {
				fmt.Printf("[INFO] pool %s 's member: %s\n", p.Name, array[1])
				continue
			}
			p.Members = append(p.Members, &Member{IP: strings.TrimSpace(array1[len(array1)-1]), Port: array2[0]})
			continue
		}
	}
}

//
type VirtualServer struct {
	Name         string
	Contents     []string
	Status       string
	Snat         string
	PoolsOrRules string
	Destination  string
	Port         string
	Protocol     string
	Profiles     []string
}

func (p *VirtualServer) Parse() {
	p.Status = "enable"
	cts := p.Contents
	if len(p.Contents) > 0 {
		cts = p.Contents[1:]
	}
	for _, v := range cts {
		if strings.Contains(v, "disable") {
			p.Status = "disable"
			continue
		}

		if strings.Contains(v, "snat") {
			array := strings.Split(strings.TrimSpace(v), spaceSep)
			if len(array) != 2 {
				fmt.Printf("[INFO] virtual server %s 's snat: %s\n", p.Name, v)
				continue
			}
			p.Snat = array[1]
			continue
		}

		if strings.Contains(v, "destination") {
			array := strings.Split(strings.TrimSpace(v), spaceSep)
			if len(array) != 2 {
				fmt.Printf("[INFO] virtual server %s 's destination: %s\n", p.Name, v)
				continue
			}
			p.Destination = array[1]
			continue
		}

		if strings.Contains(v, "ip protocol") {
			p.Protocol = strings.TrimSpace(v)
			continue
		}

		if strings.Contains(v, "pool") {
			array := strings.Split(strings.TrimSpace(v), spaceSep)
			if len(array) != 2 {
				fmt.Printf("[INFO] virtual server %s 's pool: %s\n", p.Name, v)
				continue
			}
			p.PoolsOrRules = array[1]
			continue
		}

		if strings.Contains(v, "rules") {
			array := strings.Split(strings.TrimSpace(v), spaceSep)
			if len(array) != 2 {
				fmt.Printf("[INFO] virtual server %s 's rules: %s\n", p.Name, v)
				continue
			}
			p.PoolsOrRules = array[1]
			continue
		}
	}
}

//
func (p *VirtualServer) String() string {
	return fmt.Sprintf("%s,%s,%s,%s,%s,%s", p.Name, p.Destination, p.PoolsOrRules, p.Protocol, p.Snat, p.Status)
}
