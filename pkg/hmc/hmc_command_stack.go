package hmc

import (
	"fmt"
	"strings"
)

// HmcCommandStack holds HMC commands and their options
type HmcCommandStack struct {
	HMC_CMD     map[string]string
	HMC_CMD_OPT map[string]map[string]interface{}
}

// NewHmcCommandStack creates and initializes an HmcCommandStack
func NewHmcCommandStack() *HmcCommandStack {
	return &HmcCommandStack{
		HMC_CMD: map[string]string{
			HmcCmdLshmc:       "lshmc",
			HmcCmdChhmc:       "chhmc",
			HmcCmdHmcshutdown: "hmcshutdown",
			HmcCmdGetupgfiles: "getupgfiles",
			HmcCmdUpdhmc:      "updhmc",
			HmcCmdSaveupgdata: "saveupgdata",
			HmcCmdLspwdpolicy: "lspwdpolicy",
			HmcCmdChpwdpolicy: "chpwdpolicy",
			HmcCmdRmpwdpolicy: "rmpwdpolicy",
			HmcCmdMkpwdpolicy: "mkpwdpolicy",
			HmcCmdLssyscfg:    "lssyscfg",
			HmcCmdRmsyscfg:    "rmsyscfg",
			HmcCmdMksyscfg:    "mksyscfg",
			HmcCmdChsyscfg:    "chsyscfg",
			HmcCmdChsysstate:  "chsysstate",
			HmcCmdChhwres:     "chhwres",
			HmcCmdLshwres:     "lshwres",
			HmcCmdMigrlpar:    "migrlpar",
			HmcCmdLparNetboot: "lpar_netboot",
			HmcCmdLsrefcode:   "lsrefcode",
			HmcCmdViosvrcmd:   "viosvrcmd",
			HmcCmdMkauthkeys:  "mkauthkeys",
			HmcCmdLshmcusr:    "lshmcusr",
			HmcCmdMkhmcusr:    "mkhmcusr",
			HmcCmdChhmcusr:    "chhmcusr",
			HmcCmdRmhmcusr:    "rmhmcusr",
			HmcCmdUpdlic:      "updlic",
			HmcCmdLslic:       "lslic",
			HmcCmdLssysconn:   "lssysconn",
			HmcCmdLshmcldap:   "lshmcldap",
			HmcCmdChhmcldap:   "chhmcldap",
			HmcCmdLsupgfiles:  "lsupgfiles",
			HmcCmdLsupdhmc:    "lsupdhmc",
			HmcCmdMkvterm:     "mkvterm",
			HmcCmdRmvterm:     "rmvterm",
			HmcCmdLsviosbk:    "lsviosbk",
			HmcCmdMkviosbk:    "mkviosbk",
			HmcCmdRstviosbk:   "rstviosbk",
			HmcCmdRmviosbk:    "rmviosbk",
			HmcCmdChviosbk:    "chviosbk",
			HmcCmdInstallios:  "installios",
			HmcCmdLsviosimg:   "lsviosimg",
			HmcCmdCpviosimg:   "cpviosimg",
			HmcCmdRmviosimg:   "rmviosimg",
			HmcCmdUpdvios:     "updvios",
			HmcCmdUpgvios:     "upgvios",
		},
		HMC_CMD_OPT: HmcCmdOpt, // Use the global HmcCmdOpt
	}
}

// HMC commands as constants
const (
	HmcCmdLshmc       = "LSHMC"
	HmcCmdChhmc       = "CHHMC"
	HmcCmdHmcshutdown = "HMCSHUTDOWN"
	HmcCmdGetupgfiles = "GETUPGFILES"
	HmcCmdUpdhmc      = "UPDHMC"
	HmcCmdSaveupgdata = "SAVEUPGDATA"
	HmcCmdLspwdpolicy = "LSPWDPOLICY"
	HmcCmdChpwdpolicy = "CHPWDPOLICY"
	HmcCmdRmpwdpolicy = "RMPWDPOLICY"
	HmcCmdMkpwdpolicy = "MKPWDPOLICY"
	HmcCmdLssyscfg    = "LSSYSCFG"
	HmcCmdRmsyscfg    = "RMSYSCFG"
	HmcCmdMksyscfg    = "MKSYSCFG"
	HmcCmdChsyscfg    = "CHSYSCFG"
	HmcCmdChsysstate  = "CHSYSSTATE"
	HmcCmdChhwres     = "CHHWRES"
	HmcCmdLshwres     = "LSHWRES"
	HmcCmdMigrlpar    = "MIGRLPAR"
	HmcCmdLparNetboot = "LPAR_NETBOOT"
	HmcCmdLsrefcode   = "LSREFCODE"
	HmcCmdViosvrcmd   = "VIOSVRCMD"
	HmcCmdMkauthkeys  = "MKAUTHKEYS"
	HmcCmdLshmcusr    = "LSHMCUSR"
	HmcCmdMkhmcusr    = "MKHMCUSR"
	HmcCmdChhmcusr    = "CHHMCUSR"
	HmcCmdRmhmcusr    = "RMHMCUSR"
	HmcCmdUpdlic      = "UPDLIC"
	HmcCmdLslic       = "LSLIC"
	HmcCmdLssysconn   = "LSSYSCONN"
	HmcCmdLshmcldap   = "LSHMCLDAP"
	HmcCmdChhmcldap   = "CHHMCLDAP"
	HmcCmdLsupgfiles  = "LSUPGFILES"
	HmcCmdLsupdhmc    = "LSUPDHMC"
	HmcCmdMkvterm     = "MKVTERM"
	HmcCmdRmvterm     = "RMVTERM"
	HmcCmdLsviosbk    = "LSVIOSBK"
	HmcCmdMkviosbk    = "MKVIOSBK"
	HmcCmdRstviosbk   = "RSTVIOSBK"
	HmcCmdRmviosbk    = "RMVIOSBK"
	HmcCmdChviosbk    = "CHVIOSBK"
	HmcCmdInstallios  = "INSTALLIOS"
	HmcCmdLsviosimg   = "LSVIOSIMG"
	HmcCmdCpviosimg   = "CPVIOSIMG"
	HmcCmdRmviosimg   = "RMVIOSIMG"
	HmcCmdUpdvios     = "UPDVIOS"
	HmcCmdUpgvios     = "UPGVIOS"
)

// HmcCmdOpt defines the command options and their sub-options
var HmcCmdOpt = map[string]map[string]interface{}{
	HmcCmdLshmc: {
		"-N":         " -n ",
		"-v":         " -v ",
		"-V":         " -V ",
		"-B":         " -b ",
		"-l":         " -l ",
		"-L":         " -L ",
		"-H":         " -h ",
		"-I":         " -i ",
		"-E":         " -e ",
		"-R":         " -r ",
		"--NETROUTE": " --netroute ",
		"--FIREWALL": " --firewall ",
		"-F": map[string]string{
			"POSITION":    " -F position ",
			"DESTINATION": " -F destination ",
			"GATEWAY":     " -F gateway ",
			"NETWORKMASK": " -F networkmask ",
			"INTERFACE":   " -F interface ",
			"XNTP":        " -F xntp ",
			"XNTPSERVER":  " -F xntpserver ",
			"XNTPSTATUS":  " -F xntpstatus ",
		},
		"--HELP":      " --help ",
		"--SYSLOG":    " --syslog ",
		"--NTPSERVER": " --ntpserver ",
	},
	HmcCmdChhmc: {
		"-C": map[string]string{
			"NETROUTE":    " -c netroute ",
			"SSH":         " -c ssh ",
			"XNTP":        " -c xntp ",
			"SYSLOG":      " -c syslog ",
			"NETWORK":     " -c network ",
			"DATE":        " -c date ",
			"KERBEROS":    " -c kerberos ",
			"ALTDISKBOOT": " -c altdiskboot ",
		},
		"-NM": " -nm ",
		"-S": map[string]string{
			"ADD":     " -s add ",
			"REMOVE":  " -s remove ",
			"MODIFY":  " -s modify ",
			"ENABLE":  " -s enable ",
			"DISABLE": " -s disable ",
		},
		"--ROUTETYPE": map[string]string{
			"HOST": " --routetype host ",
			"NET":  " --routetype net ",
		},
		"-G": " -g ",
		"-A": " -a ",
		"-I": " -i ",
		"-D": " -d ",
		"-T": map[string]string{
			"TCP": " -t tcp ",
			"TLS": " -t tls ",
			"UDP": " -t udp ",
		},
		"--POSITION":       " --position ",
		"-DS":              " -ds ",
		"--HELP":           " --help ",
		"-H":               " -h ",
		"--REALM":          " --realm ",
		"--DEFAULTREALM":   " --defaultrealm ",
		"--CLOCKSKEW":      " --clockskew ",
		"--TICKETLIFETIME": " --ticketlifetime ",
		"--KPASSWDADMIN":   " --kpasswdadmin ",
		"--TRACE":          " --trace ",
		"--WEAKCRYPTO":     " --weakcrypto ",
		"--FORCE":          " --force ",
		"--MODE": map[string]string{
			"INSTALL": " --mode install ",
			"UPGRADE": " --mode upgrade ",
		},
		"--IPV6AUTO": map[string]string{
			"ON":  " --ipv6auto on ",
			"OFF": " --ipv6auto off ",
		},
		"--IPV6PRIVACY": map[string]string{
			"ON":  " --ipv6privacy on ",
			"OFF": " --ipv6privacy off ",
		},
		"--IPV6DHCP": map[string]string{
			"ON":  " --ipv6dhcp on ",
			"OFF": " --ipv6dhcp off ",
		},
		"--IPV4DHCP": map[string]string{
			"ON":  " --ipv4dhcp on ",
			"OFF": " --ipv4dhcp off ",
		},
		"--LPARCOMM": map[string]string{
			"ON":  " --lparcomm on ",
			"OFF": " --lparcomm off ",
		},
		"--TSO": map[string]string{
			"ON":  " --tso on ",
			"OFF": " --tso off ",
		},
		"--SPEED": map[string]string{
			"AUTO": " --speed auto ",
			"10":   " --speed 10 ",
			"100":  " --speed 100 ",
			"1000": " --speed 1000 ",
		},
	},
	HmcCmdHmcshutdown: {
		"-T": " -t ",
		"-R": " -r ",
	},
	HmcCmdGetupgfiles: {
		"-R": map[string]string{
			"FTP":        " -r ftp ",
			"SFTP":       " -r sftp ",
			"NFS":        " -r nfs ",
			"DISK":       " -r disk ",
			"IBMWEBSITE": " -r ibmwebsite ",
		},
		"-H":       " -h ",
		"-D":       " -d ",
		"-U":       " -u ",
		"--PASSWD": " --passwd ",
		"-K":       " -k ",
		"-L":       " -l ",
		"-O":       " -o ",
		"--PTF":    " --ptf ",
	},
	HmcCmdLsupgfiles: {
		"-R": map[string]string{
			"IBMWEBSITE": " -r ibmwebsite ",
		},
	},
	HmcCmdLsupdhmc: {
		"-T": map[string]string{
			"IBMWEBSITE": " -t ibmwebsite ",
		},
	},
	HmcCmdUpdhmc: {
		"-T": map[string]string{
			"DISK":       " -t disk ",
			"DVD":        " -t dvd ",
			"FTP":        " -t ftp ",
			"SFTP":       " -t sftp ",
			"NFS":        " -t nfs ",
			"USB":        " -t usb ",
			"IBMWEBSITE": " -t ibmwebsite ",
		},
		"-F":       " -f ",
		"-R":       " -r ",
		"-C":       " -c ",
		"-N":       " -n ",
		"-H":       " -h ",
		"-D":       " -d ",
		"-U":       " -u ",
		"--PASSWD": " --passwd ",
		"-K":       " -k ",
		"-L":       " -l ",
		"-O":       " -o ",
		"--PTF":    " --ptf ",
	},
	HmcCmdSaveupgdata: {
		"-R": map[string]string{
			"DISK":     " -r disk ",
			"DISKUSB":  " -r diskusb ",
			"DISKFTP":  " -r diskftp ",
			"DISKSFTP": " -r disksftp ",
		},
		"-H":        " -h ",
		"-U":        " -u ",
		"-K":        " -k ",
		"-D":        " -d ",
		"-I":        " -i ",
		"--PASSWD":  " --passwd ",
		"--MIGRATE": " --migrate ",
		"--FORCE":   " --force ",
	},
	HmcCmdLspwdpolicy: {
		"-T": map[string]string{
			"P": " -t p ",
			"S": " -t s ",
		},
	},
	HmcCmdMkpwdpolicy: {
		"-I": map[string]string{
			"NAME":                "name",
			"DESCRIPTION":         "description",
			"MIN_PWAGE":           "min_pwage",
			"PWAGE":               "pwage",
			"WARN_PWAGE":          "warn_pwage",
			"MIN_LENGTH":          "min_length",
			"HIST_SIZE":           "hist_size",
			"MIN_DIGITS":          "min_digits",
			"MIN_UPPERCASE_CHARS": "min_uppercase_chars",
			"MIN_LOWERCASE_CHARS": "min_lowercase_chars",
			"MIN_SPECIAL_CHARS":   "min_special_chars",
		},
	},
	HmcCmdChpwdpolicy: {
		"-O": map[string]string{
			"A": " -o a ",
			"D": " -o d ",
			"M": " -o m ",
		},
		"-N": " -n ",
		"-I": map[string]string{
			"NAME":                "name",
			"DESCRIPTION":         "description",
			"MIN_PWAGE":           "min_pwage",
			"PWAGE":               "pwage",
			"WARN_PWAGE":          "warn_pwage",
			"MIN_LENGTH":          "min_length",
			"HIST_SIZE":           "hist_size",
			"MIN_DIGITS":          "min_digits",
			"MIN_UPPERCASE_CHARS": "min_uppercase_chars",
			"MIN_LOWERCASE_CHARS": "min_lowercase_chars",
			"MIN_SPECIAL_CHARS":   "min_special_chars",
			"NEW_NAME":            "new_name",
		},
	},
	HmcCmdRmpwdpolicy: {
		"-N": " -n ",
	},
	HmcCmdLssyscfg: {
		"-R": map[string]string{
			"LPAR":    " -r lpar",
			"SYS":     " -r sys",
			"PROF":    " -r prof",
			"SYSPROF": " -r sysprof",
		},
		"-M": " -m ",
		"-F": " -F ",
		"--FILTER": map[string]string{
			"LPAR_NAMES":    "lpar_names",
			"LPAR_IDS":      "lpar_ids",
			"WORK_GROUPS":   "work_groups",
			"PROFILE_NAMES": "profile_names",
		},
	},
	HmcCmdLshwres: {
		"-R":      " -r ",
		"-M":      " -m ",
		"--LEVEL": " --level ",
		"-F":      " -F ",
	},
	HmcCmdRmsyscfg: {
		"-R": map[string]string{
			"LPAR": " -r lpar",
		},
		"-M":      " -m ",
		"-N":      " -n ",
		"--ID":    " --id ",
		"VIOSCFG": " --vioscfg",
		"VDISKS":  " --vdisk",
	},
	HmcCmdMksyscfg: {
		"-R": map[string]string{
			"LPAR":    " -r lpar",
			"PROF":    " -r prof",
			"SYSPROF": " -r sysprof",
		},
		"-M":   " -m ",
		"--ID": " --id ",
		"-I": map[string]string{
			"NAME":                              "name",
			"PROFILE_NAME":                      "profile_name",
			"LPAR_ENV":                          "lpar_env",
			"MIN_MEM":                           "min_mem",
			"DESIRED_MEM":                       "desired_mem",
			"MAX_MEM":                           "max_mem",
			"MIN_PROCS":                         "min_procs",
			"MAX_PROCS":                         "max_procs",
			"MIN_PROC_UNITS":                    "min_proc_units",
			"DESIRED_PROC_UNITS":                "desired_proc_units",
			"MAX_PROC_UNITS":                    "max_proc_units",
			"DESIRED_PROCS":                     "desired_procs",
			"BOOT_MODE":                         "boot_mode",
			"SHARING_MODE":                      "sharing_mode",
			"PROC_MODE":                         "proc_mode",
			"MAX_VIRTUAL_SLOTS":                 "max_virtual_slots",
			"VIRTUAL_SCSI_ADAPTERS":             "virtual_scsi_adapters",
			"VIRTUAL_ETH_ADAPTERS":              "virtual_eth_adapters",
			"CONSOLE_SLOT":                      "console_slot",
			"LPAR_NAME":                         "lpar_name",
			"ALL_RESOURCES":                     "all_resources",
			"LPAR_NAMES":                        "lpar_names",
			"LPAR_IDS":                          "lpar_ids",
			"PROFILE_NAMES":                     "profile_names",
			"HPT_RATIO":                         "hpt_ratio",
			"VTPM_ENABLED":                      "vtpm_enabled",
			"REMOTE_RESTART_CAPABLE":            "remote_restart_capable",
			"MEM_EXPANSION":                     "mem_expansion",
			"SUSPEND_CAPABLE":                   "suspend_capable",
			"OS400_RESTRICTED_IO_MODE":          "os400_restricted_io_mode",
			"SHARED_PROC_POOL_ID":               "shared_proc_pool_id",
			"SIMPLIFIED_REMOTE_RESTART_CAPABLE": "simplified_remote_restart_capable",
			"MEM_MODE":                          "mem_mode",
			"ALT_RESTART_DEVICE_SLOT":           "alt_restart_device_slot",
			"LPAR_PROC_COMPAT_MODE":             "lpar_proc_compat_mode",
			"PPT_RATIO":                         "ppt_ratio",
			"SECURE_BOOT":                       "secure_boot",
			"LPAR_ID":                           "lpar_id",
			"UNCAP_WEIGHT":                      "uncap_weight",
			"TIME_REF":                          "time_ref",
			"PRIMARY_RS_VIOS_ID":                "primary_rs_vios_id",
			"PRIMARY_RS_VIOS_NAME":              "primary_rs_vios_name",
			"SECONDARY_RS_VIOS_NAME":            "secondary_rs_vios_name",
			"SECONDARY_RS_VIOS_ID":              "secondary_rs_vios_id",
			"ALLOW_PERF_COLLECTION":             "allow_perf_collection",
			"HARDWARE_MEM_EXPANSION":            "hardware_mem_expansion",
			"HARDWARE_MEM_ENCRYPTION":           "hardware_mem_encryption",
			"SYNC_CURR_PROFILE":                 "sync_curr_profile",
			"LPAR_AVAIL_PRIORITY":               "lpar_avail_priority",
			"MIGRATION_DISABLED":                "migration_disabled",
			"RS_DEVICE_NAME":                    "rs_device_name",
			"MSP":                               "msp",
			"MIN_NUM_HUGE_PAGES":                "min_num_huge_pages",
			"DESIRED_NUM_HUGE_PAGES":            "desired_num_huge_pages",
			"MAX_NUM_HUGE_PAGES":                "max_num_huge_pages",
			"DESIRED_IO_ENTITLED_MEM":           "desired_io_entitled_mem",
			"PRIMARY_PAGING_VIOS_NAME":          "primary_paging_vios_name",
			"PRIMARY_PAGING_VIOS_ID":            "primary_paging_vios_id",
			"SECONDARY_PAGING_VIOS_NAME":        "secondary_paging_vios_name",
			"SECONDARY_PAGING_VIOS_ID":          "secondary_paging_vios_id",
			"BSR_ARRAYS":                        "bsr_arrays",
			"MEM_WEIGHT":                        "mem_weight",
			"AFFINITY_GROUP_ID":                 "affinity_group_id",
			"IO_SLOTS":                          "io_slots",
			"AUTO_START":                        "auto_start",
			"POWER_CTRL_LPAR_IDS":               "power_ctrl_lpar_ids",
			"POWER_CTRL_LPAR_NAMES":             "power_ctrl_lpar_names",
			"CONN_MONITORING":                   "conn_monitoring",
			"LPAR_IO_POOL_IDS":                  "lpar_io_pool_ids",
			"VIRTUAL_ETH_VSI_PROFILES":          "virtual_eth_vsi_profiles",
			"VIRTUAL_FC_ADAPTERS":               "virtual_fc_adapters",
			"VIRTUAL_SERIAL_ADAPTERS":           "virtual_serial_adapters",
			"VIRTUAL_OPTI_POOL_ID":              "virtual_opti_pool_id",
			"HSL_POOL_ID":                       "hsl_pool_id",
			"ALT_CONSOLE_SLOT":                  "alt_console_slot",
			"OP_CONSOLE_SLOT":                   "op_console_slot",
			"LOAD_SOURCE_SLOT":                  "load_source_slot",
			"KEYSTORE_KBYTES":                   "keystore_kbytes",
			"SRIOV_ROCE_LOGICAL_PORTS":          "sriov_roce_logical_ports",
			"POWERVM_MGMT_CAPABLE":              "powervm_mgmt_capable",
			"SRIOV_ETH_LOGICAL_PORTS":           "sriov_eth_logical_ports",
			"REDUNDANT_ERR_PATH_REPORTING":      "redundant_err_path_reporting",
			"WORK_GROUP_ID":                     "work_group_id",
			"VIRTUAL_SERIAL_NUM":                "virtual_serial_num",
			"SHARED_PROC_POOL_NAME":             "shared_proc_pool_name",
			"VTPM_VERSION":                      "vtpm_version",
			"VTPM_ENCRYPTION":                   "vtpm_encryption",
		},
		"-P": " -p ",
		"-N": " -n ",
		"-O": map[string]string{
			"SAVE": " -o save",
		},
		"--FORCE": " --force ",
	},
	HmcCmdChsyscfg: {
		"-R": map[string]string{
			"LPAR":    " -r lpar",
			"PROF":    " -r prof",
			"SYSPROF": " -r sysprof",
			"SYS":     " -r sys",
		},
		"-M": " -m ",
		"-N": " -n ",
		"-P": " -p ",
		"-O": map[string]string{
			"APPLY": " -o apply",
		},
		"--FORCE": " --force ",
		"-I": map[string]string{
			"NAME":                              "name",
			"PROFILE_NAME":                      "profile_name",
			"LPAR_ENV":                          "lpar_env",
			"MIN_MEM":                           "min_mem",
			"DESIRED_MEM":                       "desired_mem",
			"MAX_MEM":                           "max_mem",
			"MIN_PROCS":                         "min_procs",
			"MAX_PROCS":                         "max_procs",
			"MIN_PROC_UNITS":                    "min_proc_units",
			"DESIRED_PROC_UNITS":                "desired_proc_units",
			"MAX_PROC_UNITS":                    "max_proc_units",
			"DESIRED_PROCS":                     "desired_procs",
			"BOOT_MODE":                         "boot_mode",
			"SHARING_MODE":                      "sharing_mode",
			"PROC_MODE":                         "proc_mode",
			"LPAR_NAME":                         "lpar_name",
			"VIRTUAL_SCSI_ADAPTERS":             "virtual_scsi_adapters",
			"VIRTUAL_ETH_ADAPTERS":              "virtual_eth_adapters",
			"VIRTUAL_FC_ADAPTERS":               "virtual_fc_adapters",
			"MAX_VIRTUAL_SLOTS":                 "max_virtual_slots",
			"LPAR_NAMES":                        "lpar_names",
			"AUTO_PRIORITY_FAILOVER":            "auto_priority_failover",
			"PROFILE_NAMES":                     "profile_names",
			"VTPM_ENABLED":                      "vtpm_enabled",
			"VTPM_STATE":                        "vtpm_state",
			"LPAR_PROC_COMPAT_MODE":             "lpar_proc_compat_mode",
			"HPT_RATIO":                         "hpt_ratio",
			"SUSPEND_CAPABLE":                   "suspend_capable",
			"REMOTE_RESTART_CAPABLE":            "remote_restart_capable",
			"SIMPLIFIED_REMOTE_RESTART_CAPABLE": "simplified_remote_restart_capable",
			"SYNC_CURR_PROFILE":                 "sync_curr_profile",
			"PPT_RATIO":                         "ppt_ratio",
			"CONSOLE_SLOT":                      "console_slot",
			"SECURE_BOOT":                       "secure_boot",
			"KEYSTORE_KBYTES":                   "keystore_kbytes",
			"LOAD_SOURCE_SLOT":                  "load_source_slot",
			"ALT_RESTART_DEVICE_SLOT":           "alt_restart_device_slot",
			"IPL_SOURCE":                        "ipl_source",
			"VIRTUAL_SERIAL_NUM":                "virtual_serial_num",
			"VTPM_VERSION":                      "vtpm_version",
			"VTPM_ENCRYPTION":                   "vtpm_encryption",
			"NEW_NAME":                          "new_name",
			"POWER_OFF_POLICY":                  "power_off_policy",
			"POWER_ON_LPAR_START_POLICY":        "power_on_lpar_start_policy",
		},
	},
	HmcCmdChsysstate: {
		"-R": map[string]string{
			"LPAR":    " -r lpar",
			"SYS":     " -r sys",
			"SYSPROF": " -r sysprof",
		},
		"-M": " -m ",
		"-O": map[string]string{
			"ON":             " -o on ",
			"ONSTANDBY":      " -o onstandby ",
			"ONSTARTPOLICY":  " -o onstartpolicy ",
			"ONSYSPROF":      " -o onsysprof ",
			"ONHWDISK":       " -o onhwdisc ",
			"OFF":            " -o off ",
			"REBUILD":        " -o rebuild ",
			"RECOVER":        " -o recover ",
			"SPFAILOVER":     " -o spfailover ",
			"CHKEY":          " -o chkey ",
			"SHUTDOWN":       " -o shutdown ",
			"OSSHUTDOWN":     " -o osshutdown ",
			"DUMPRESTART":    " -o dumprestart ",
			"RETRYDUMP":      " -o retrydump ",
			"DSTON":          " -o dston ",
			"REMOTEDSTOFF":   " -o remotedstoff ",
			"REMOTEDSTON":    " -o remotedston ",
			"CONSOLESERVICE": " -o consoleservice ",
			"IOPRESET":       " -o iopreset ",
			"IOPDUMP":        " -o iopdump ",
			"UNOWNEDIOOFF":   " -o unownediooff ",
		},
		"-F": " -f ",
		"-K": map[string]string{
			"MANUAL": " -k manual ",
			"NORM":   " -k norm ",
		},
		"-B": map[string]string{
			"NORM":         " -b norm ",
			"DIAG_DEFAULT": " -b dd ",
			"DIAG_STORED":  " -b ds ",
			"OK":           " -b of ",
			"SMS":          " -b sms ",
		},
		"--IMMED": " --immed ",
		"-I": map[string]string{
			"A": " -i a ",
			"B": " -i b ",
			"C": " -i c ",
			"D": " -i d ",
		},
		"--RESTART":   " --restart ",
		"-N":          " -n ",
		"--ID":        " --id ",
		"--IP":        " --ip ",
		"--GATEWAY":   " --gateway ",
		"--SERVERIP":  " --serverip ",
		"--SERVERDIR": " --serverdir ",
		"--SPEED": map[string]string{
			"AUTO": " --speed auto ",
			"1":    " --speed 1 ",
			"10":   " --speed 10 ",
			"100":  " --speed 100 ",
			"1000": " --speed 1000 ",
		},
		"--DUPLEX": map[string]string{
			"AUTO": " --duplex auto ",
			"HALF": " --duplex half ",
			"FULL": " --duplex full ",
		},
		"--MTU": map[string]string{
			"1500": " --mtu 1500 ",
			"9000": " --mtu 9000 ",
		},
		"--VLAN": " --vlan ",
	},
	HmcCmdChhwres: {
		"-R": map[string]string{
			"MEM": " -r mem ",
		},
		"-M": " -m ",
		"-O": map[string]string{
			"S": " -o s ",
		},
		"-A": map[string]string{
			"REQUESTED_NUM_SYS_HUGE_PAGES": "requested_num_sys_huge_pages",
			"PEND_MEM_REGION_SIZE":         "pend_mem_region_size",
			"MEM_MIRRORING_MODE":           "mem_mirroring_mode",
		},
	},
	HmcCmdMigrlpar: {
		"-O": map[string]string{
			"V": " -o v",
			"M": " -o m",
			"R": " -o r",
		},
		"-M":    " -m ",
		"-T":    " -t ",
		"-P":    " -p ",
		"--IP":  " --ip ",
		"--ALL": " --all",
		"--ID":  " --id ",
		"-W":    " -w ",
		"-I":    " -i ",
	},
	HmcCmdLparNetboot: {
		"-A": " -A",
		"-M": " -M",
		"-D": " -D",
		"-N": " -n",
		"-T": " -t ",
		"-S": " -S ",
		"-G": " -G ",
		"-C": " -C ",
		"-F": " -f",
		"-L": " -l ",
		"-V": " -V ",
		"-Y": " -Y ",
		"-K": " -K ",
		"-x": " -x ",
		"-v": " -v ",
		"-i": " -i ",
		"-d": " -d ",
		"-s": " -s ",
		"-m": " -m ",
	},
	HmcCmdLsrefcode: {
		"-R": map[string]string{
			"LPAR": " -r lpar",
		},
		"-M": " -m ",
		"-F": " -F ",
		"--FILTER": map[string]string{
			"LPAR_NAMES": "lpar_names",
		},
	},
	HmcCmdViosvrcmd: {
		"-M":      " -m ",
		"-P":      " -p ",
		"-C":      " -c ",
		"--ID":    " --id ",
		"--ADMIN": " --admin ",
	},
	HmcCmdMkauthkeys: {
		"-G":       " -g ",
		"--IP":     " --ip ",
		"-U":       " -u ",
		"--PASSWD": " --passwd ",
		"--TEST":   " --test",
	},
	HmcCmdLshmcusr: {
		"-T": map[string]string{
			"DEFAULT": " -t default ",
			"USER":    " -t user ",
		},
		"--FILTER": map[string]string{
			"NAMES":                "names",
			"RESOURCES":            "resources",
			"RESOURCEROLES":        "resourceroles",
			"TASKROLES":            "taskroles",
			"PASSWORD_ENCRYPTIONS": "password_encryptions",
		},
	},
	HmcCmdRmhmcusr: {
		"-U": " -u ",
		"-T": map[string]string{
			"ALL":        " -t all ",
			"LOCAL":      " -t local ",
			"KERBEROS":   " -t kerberos ",
			"LDAP":       " -t ldap ",
			"AUTOMANAGE": " -t automanage ",
		},
	},
	HmcCmdMkhmcusr: {
		"-I": map[string]string{
			"NAME":                  "name",
			"TASKROLE":              "taskrole",
			"RESOURCEROLE":          "resourcerole",
			"DESCRIPTION":           "description",
			"PASSWD":                "passwd",
			"PWAGE":                 "pwage",
			"MIN_PWAGE":             "min_pwage",
			"AUTHENTICATION_TYPE":   "authentication_type",
			"SESSION_TIMEOUT":       "session_timeout",
			"VERIFY_TIMEOUT":        "verify_timeout",
			"IDLE_TIMEOUT":          "idle_timeout",
			"REMOTE_WEBUI_ACCESS":   "remote_webui_access",
			"REMOTE_SSH_ACCESS":     "remote_ssh_access",
			"REMOTE_USER_NAME":      "remote_user_name",
			"INACTIVITY_EXPIRATION": "inactivity_expiration",
		},
	},
	HmcCmdChhmcusr: {
		"-T": map[string]string{
			"DEFAULT": " -t default ",
		},
		"-O": map[string]string{
			"A": " -o a ",
			"E": " -o e ",
		},
		"-U": " -u ",
		"-I": map[string]string{
			"NAME":                         "name",
			"TASKROLE":                     "taskrole",
			"RESOURCEROLE":                 "resourcerole",
			"DESCRIPTION":                  "description",
			"PASSWD":                       "passwd",
			"PWAGE":                        "pwage",
			"MIN_PWAGE":                    "min_pwage",
			"AUTHENTICATION_TYPE":          "authentication_type",
			"SESSION_TIMEOUT":              "session_timeout",
			"VERIFY_TIMEOUT":               "verify_timeout",
			"IDLE_TIMEOUT":                 "idle_timeout",
			"REMOTE_WEBUI_ACCESS":          "remote_webui_access",
			"REMOTE_SSH_ACCESS":            "remote_ssh_access",
			"REMOTE_USER_NAME":             "remote_user_name",
			"INACTIVITY_EXPIRATION":        "inactivity_expiration",
			"NEW_NAME":                     "new_name",
			"CURRENT_PASSWD":               "current_passwd",
			"PASSWD_AUTHENTICATION":        "passwd_authentication",
			"MAX_WEBUI_LOGIN_SUSPEND_TIME": "max_webui_login_suspend_time",
			"MAX_WEBUI_LOGIN_ATTEMPTS":     "max_webui_login_attempts",
			"WEBUI_LOGIN_SUSPEND_TIME":     "webui_login_suspend_time",
		},
	},
	HmcCmdUpdlic: {
		"-M": " -m ",
		"-O": map[string]string{
			"RETINSTACT": " -o a",
			"UPGRADE":    " -o u",
			"ACCEPT":     " -o c",
		},
		"-T": map[string]string{
			"SYS": " -t sys",
			"BMC": " -t bmc",
		},
		"-L":       " -l ",
		"-R":       " -r ",
		"-H":       " -h ",
		"-U":       " -u ",
		"--PASSWD": " --passwd ",
		"-K":       " -k ",
		"-D":       " -d ",
	},
	HmcCmdLslic: {
		"-M": " -m ",
		"-F": map[string]string{
			"SPNAMELEVEL": " -F activated_spname,activated_level,ecnumber",
		},
	},
	HmcCmdLssysconn: {
		"-R": map[string]string{
			"ALL": " -r all",
		},
		"-F": map[string]string{
			"MTMS": " -F type_model_serial_num",
		},
	},
	HmcCmdLshmcldap: {
		"-R": map[string]string{
			"CONFIG": " -r config",
			"USER":   " -r user",
		},
		"--FILTER": map[string]string{
			"NAMES": " names",
		},
	},
	HmcCmdChhmcldap: {
		"-O": map[string]string{
			"S": " -o s",
			"R": " -o r",
		},
		"-R": map[string]string{
			"BACKUP":                " -r backup",
			"LDAP":                  " -r ldap",
			"BINDDN":                " -r binddn",
			"BINDPW":                " -r bindpw",
			"SEARCHFILTER":          " -r searchfilter",
			"HMCGROUPS":             " -r hmcgroups",
			"GROUPMEMBERATTRIBUTES": " -r groupmemberattributes",
		},
		"PRIMARY": " --primary",
		"BACKUP":  " --backup",
		"AUTOMANAGE": map[string]string{
			"0": " --automanage 0",
			"1": " --automanage 1",
		},
		"BINDPW":               " --bindpw",
		"BASEDN":               " --basedn",
		"BINDDN":               " --binddn",
		"HMCAUTHNAMEATTRIBUTE": " --hmcauthnameattribute",
		"STARTTLS": map[string]string{
			"0": " --starttls 0",
			"1": " --starttls 1",
		},
		"AUTH": map[string]string{
			"LDAP":     " --auth ldap",
			"KERBEROS": " --auth kerberos",
		},
		"TIMELIMIT":             " --timelimit",
		"BINDTIMELIMIT":         " --bindtimelimit",
		"LOGINATTRIBUTE":        " --loginattribute",
		"HMCUSERPROPSATTRIBUTE": " --hmcuserpropsattribute",
		"HMCAUTHATTRIBUTE":      " --hmcauthnameattribute",
		"SEARCHFILTER":          " --searchfilter",
		"SCOPE": map[string]string{
			"ONE": " --scope one",
			"SUB": " --scope sub",
		},
		"REFERRALS": map[string]string{
			"1": " --referrals 1",
			"0": " --referrals 0",
		},
		"HMCGROUPS": " --hmcgroups",
		"AUTHSEARCH": map[string]string{
			"BASE": " --authsearch base",
			"NONE": " --authsearch none",
		},
		"TLSREQCERT": map[string]string{
			"NEVER":  " --tlsreqcert never",
			"ALLOW":  " --tlsreqcert allow",
			"TRY":    " --tlsreqcert try",
			"DEMAND": " --tlsreqcert demand",
		},
		"GROUPATTRIBUTE":  " --groupattribute",
		"MEMBERATTRIBUTE": " --memberattribute",
	},
	HmcCmdMkvterm: {
		"-m": " -m ",
		"-p": " -p ",
	},
	HmcCmdRmvterm: {
		"-m": " -m ",
		"-p": " -p ",
	},
	HmcCmdLsviosbk: {
		"--FILTER": map[string]string{
			"VIOS_NAMES": "vios_names",
			"SYS_NAMES":  "sys_names",
			"TYPES":      "types",
			"VIOS_UUIDS": "vios_uuids",
			"VIOS_IDS":   "vios_ids",
		},
	},
	HmcCmdMkviosbk: {
		"-T":     " -t ",
		"-M":     " -m ",
		"-P":     " -p ",
		"-F":     " -f ",
		"-A":     " -a ",
		"--ID":   " --id ",
		"--UUID": " --uuid ",
	},
	HmcCmdRstviosbk: {
		"-T":     " -t ",
		"-M":     " -m ",
		"-P":     " -p ",
		"-F":     " -f ",
		"--ID":   " --id ",
		"--UUID": " --uuid ",
		"-R":     " -r ",
	},
	HmcCmdRmviosbk: {
		"-T":     " -t ",
		"-M":     " -m ",
		"-P":     " -p ",
		"-F":     " -f ",
		"--ID":   " --id ",
		"--UUID": " --uuid ",
	},
	HmcCmdChviosbk: {
		"-T":     " -t ",
		"-M":     " -m ",
		"-P":     " -p ",
		"-F":     " -f ",
		"--ID":   " --id ",
		"--UUID": " --uuid ",
		"-O":     " -o ",
		"-A":     " -a ",
	},
	HmcCmdInstallios: {
		"-D": " -d ",
		"-I": " -i ",
		"-G": " -g ",
		"-S": " -S ",
		"-M": " -m ",
		"-s": " -s ",
		"-P": " -p ",
		"-r": " -r ",
		"-R": " -R ",
	},
	HmcCmdCpviosimg: {
		"-R": map[string]string{
			"SFTP": " -r sftp ",
			"NFS":  " -r nfs ",
		},
		"-N":        " -n ",
		"-H":        " -h ",
		"-U":        " -u ",
		"-F":        " -f ",
		"--PASSWD":  " --passwd ",
		"-K":        " -k ",
		"-D":        " -d ",
		"-L":        " -l ",
		"--OPTIONS": " --options ",
	},
	HmcCmdRmviosimg: {
		"-N": " -n ",
	},
	HmcCmdUpdvios: {
		"-R":        " -r ",
		"-M":        " -m ",
		"-P":        " -p ",
		"--ID":      " --id ",
		"-N":        " -n ",
		"-F":        " -f ",
		"-H":        " -h ",
		"-U":        " -u ",
		"--PASSWD":  " --passwd ",
		"-K":        " -k ",
		"-D":        " -d ",
		"-L":        " -l ",
		"--OPTIONS": " --options ",
		"--RESTART": " --restart ",
		"--SAVE":    " --save ",
		"--DISK":    " --disk ",
	},
}

// FilterBuilder builds the --filter option string for a command
func FilterBuilder(cmdKey string, configOptions map[string]string) (string, error) {
	if _, ok := HmcCmdOpt[cmdKey]; !ok {
		return "", fmt.Errorf("invalid command key: %s", cmdKey)
	}
	if _, ok := HmcCmdOpt[cmdKey]["--FILTER"]; !ok {
		return "", fmt.Errorf("command %s does not support --FILTER", cmdKey)
	}
	attribute, ok := HmcCmdOpt[cmdKey]["--FILTER"].(map[string]string)
	if !ok {
		return "", fmt.Errorf("--FILTER for %s is not a map", cmdKey)
	}

	configStr := " --filter \""
	for key, value := range configOptions {
		if _, ok := attribute[key]; !ok {
			return "", fmt.Errorf("invalid filter attribute: %s", key)
		}
		if strings.Contains(value, ",") {
			configStr += fmt.Sprintf("\"%s=%s\",", attribute[key], value)
		} else {
			configStr += fmt.Sprintf("%s=%s,", attribute[key], value)
		}
	}
	configStr = strings.TrimSuffix(configStr, ",")
	configStr += "\""
	return configStr, nil
}

// ConfigBuilder builds the command configuration string
func ConfigBuilder(cmdKey string, configOptions map[string]interface{}) (string, error) {
	if _, ok := HmcCmdOpt[cmdKey]; !ok {
		return "", fmt.Errorf("invalid command key: %s", cmdKey)
	}
	attribute := HmcCmdOpt[cmdKey]
	var cmdStr strings.Builder

	for key, value := range configOptions {
		if key == "--FILTER" {
			if filterOptions, ok := value.(map[string]string); ok {
				filterStr, err := FilterBuilder(cmdKey, filterOptions)
				if err != nil {
					return "", err
				}
				cmdStr.WriteString(filterStr)
			} else {
				return "", fmt.Errorf("--FILTER value must be a map")
			}
			continue
		}
		if attr, ok := attribute[key]; ok {
			switch v := attr.(type) {
			case string:
				if strVal, ok := value.(string); ok {
					cmdStr.WriteString(v + " " + strVal)
				} else {
					return "", fmt.Errorf("value for %s must be a string", key)
				}
			case map[string]string:
				if strVal, ok := value.(string); ok {
					if opt, ok := v[strings.ToUpper(strVal)]; ok {
						cmdStr.WriteString(opt)
					} else {
						return "", fmt.Errorf("invalid value %s for %s", strVal, key)
					}
				} else {
					return "", fmt.Errorf("value for %s must be a string", key)
				}
			default:
				return "", fmt.Errorf("unsupported attribute type for %s", key)
			}
		} else {
			return "", fmt.Errorf("invalid option: %s", key)
		}
	}
	return strings.TrimSpace(cmdStr.String()), nil
}

// ParseColonSV parses colon-separated values into a map
func ParseColonSV(colonSVData string) (map[string]string, error) {
	innerDict := make(map[string]string)
	for _, each := range strings.Split(colonSVData, ": ") {
		each = strings.Trim(each, "\"")
		keyValue := strings.SplitN(each, "=", 2)
		if len(keyValue) != 2 {
			return nil, fmt.Errorf("invalid colon-separated data: %s", each)
		}
		innerDict[strings.ToUpper(keyValue[0])] = keyValue[1]
	}
	return innerDict, nil
}

// ParseCSV parses comma-separated values into a map
func ParseCSV(csvData string, userConfig map[string]interface{}) (map[string]interface{}, error) {
	dict := make(map[string]interface{})
	var keyBkup, valueBkup string
	valueHasColonDelim := false
	caseLshmccert := false
	var prevData string

	csvList := strings.Split(csvData, ",")
	for i, each := range csvList {
		var nextData string
		if i+1 < len(csvList) {
			nextData = csvList[i+1]
		}

		each = strings.TrimSpace(each)
		if valueHasColonDelim {
			if !strings.Contains(each, "=") {
				valueBkup += "," + each
				continue
			}
			if strings.Contains(each, ": ") && !strings.Contains(strings.Split(each, ": ")[0], "=") {
				valueBkup += "," + each
				if nextData == "" {
					colonDict, err := ParseColonSV(valueBkup)
					if err != nil {
						return nil, err
					}
					dict[keyBkup] = append(dict[keyBkup].([]map[string]string), colonDict)
				}
				continue
			}
			if strings.Contains(each, ": ") && strings.HasPrefix(each, "\"") {
				if valueBkup != "" {
					colonDict, err := ParseColonSV(valueBkup)
					if err != nil {
						return nil, err
					}
					dict[keyBkup] = append(dict[keyBkup].([]map[string]string), colonDict)
				}
				valueBkup = strings.Trim(each, "\"")
				continue
			}
			if strings.Contains(each, ": ") {
				if valueBkup != "" {
					colonDict, err := ParseColonSV(valueBkup)
					if err != nil {
						return nil, err
					}
					dict[keyBkup] = append(dict[keyBkup].([]map[string]string), colonDict)
				}
				valueBkup = ""
				if !strings.Contains(nextData, ": ") && strings.Contains(nextData, "=") {
					colonDict, err := ParseColonSV(each)
					if err != nil {
						return nil, err
					}
					dict[keyBkup] = append(dict[keyBkup].([]map[string]string), colonDict)
					valueHasColonDelim = false
				}
				continue
			}
			if valueBkup != "" {
				colonDict, err := ParseColonSV(valueBkup)
				if err != nil {
					return nil, err
				}
				dict[keyBkup] = append(dict[keyBkup].([]map[string]string), colonDict)
			}
			keyValue := strings.SplitN(each, "=", 2)
			if len(keyValue) != 2 {
				return nil, fmt.Errorf("invalid CSV data: %s", each)
			}
			key := strings.Trim(strings.Trim(keyValue[0], "\""), "\r\n")
			value := strings.Trim(keyValue[1], "\r\n")
			dict[strings.ToUpper(key)] = value
			valueHasColonDelim = false
			keyBkup = strings.ToUpper(key)
			valueBkup = value
		} else {
			if caseLshmccert {
				key := keyBkup
				value := dict[keyBkup].(string) + "," + strings.Trim(each, "\"")
				if strings.HasSuffix(each, "\"") {
					caseLshmccert = false
				}
				dict[key] = value
				keyBkup = key
				valueBkup = value
			} else if strings.Contains(each, "<") && (strings.Contains(each, ">") || (i+1 < len(csvList) && strings.Contains(csvList[i+1], ">"))) {
				t := strings.SplitN(each, "=", 2)
				var key, value string
				if t[0][0] != '<' {
					key = t[0]
					value = strings.Join(t[1:], "=")
					value = strings.Trim(value, "\"")
				} else {
					key = keyBkup
					value = dict[keyBkup].(string) + "," + strings.Join(t, "=")
				}
				dict[strings.ToUpper(key)] = value
				keyBkup = strings.ToUpper(key)
				valueBkup = value
			} else if strings.Contains(prevData, "<") && strings.Contains(each, ">") {
				key := keyBkup
				value := dict[keyBkup].(string) + "," + strings.Trim(each, "\"")
				dict[key] = value
				keyBkup = key
				valueBkup = value
			} else if strings.HasPrefix(each, "\"") && len(strings.SplitN(each, "=", 3)) == 3 {
				t := strings.SplitN(each, "=", 3)
				key := strings.Trim(t[0], "\"")
				value := t[1] + "=" + t[2]
				dict[strings.ToUpper(key)] = value
				caseLshmccert = true
				keyBkup = strings.ToUpper(key)
				valueBkup = value
			} else {
				keyValue := strings.SplitN(each, "=", 2)
				if len(keyValue) != 2 {
					if strings.Contains(each, ": ") {
						valueHasColonDelim = true
						parts := strings.SplitN(each, "=", 2)
						if len(parts) < 2 {
							return nil, fmt.Errorf("invalid CSV data: %s", each)
						}
						key := strings.Trim(parts[0], "\"")
						dict[strings.ToUpper(key)] = []map[string]string{}
						value := strings.TrimPrefix(strings.Trim(each, "\""), key+"=")
						if nextData == "" {
							colonDict, err := ParseColonSV(value)
							if err != nil {
								return nil, err
							}
							dict[strings.ToUpper(key)] = []map[string]string{colonDict}
						} else {
							valueBkup = value
							keyBkup = strings.ToUpper(key)
						}
						continue
					}
					return nil, fmt.Errorf("invalid CSV data: %s", each)
				}
				key := strings.Trim(strings.Trim(keyValue[0], "\""), "\r\n")
				value := strings.Trim(keyValue[1], "\r\n")
				dict[strings.ToUpper(key)] = value
				keyBkup = strings.ToUpper(key)
				valueBkup = value
			}
		}
		prevData = each
	}
	return dict, nil
}

// ParseMultiLineCSV parses multi-line CSV data into a slice of maps
func ParseMultiLineCSV(csvData string, userConfig map[string]interface{}) ([]map[string]interface{}, error) {
	var listOfDict []map[string]interface{}
	lines := strings.Split(csvData, "\n")
	for _, line := range lines {
		if line == "" {
			continue
		}
		dict, err := ParseCSV(line, userConfig)
		if err != nil {
			return nil, err
		}
		listOfDict = append(listOfDict, dict)
	}
	return listOfDict, nil
}

// ParseAttributes parses attributes and values into a map
func ParseAttributes(csvAttrStr, csvValueStr string) (map[string]string, error) {
	attrs := strings.Split(csvAttrStr, ",")
	values := strings.Split(strings.TrimSpace(csvValueStr), ",")

	if strings.Contains(csvValueStr, ",\"") || strings.Contains(csvValueStr, "\"") {
		var csvValueList []string
		var finalList []string
		startComma := false
		for _, each := range values {
			if startComma && strings.HasSuffix(each, "\"") {
				csvValueList = append(csvValueList, strings.Trim(each, "\""))
				startComma = false
				finalList = append(finalList, strings.Join(csvValueList, ","))
				csvValueList = nil
			} else if strings.HasPrefix(each, "\"") {
				csvValueList = append(csvValueList, strings.Trim(each, "\""))
				startComma = true
			} else if startComma {
				csvValueList = append(csvValueList, each)
			} else {
				finalList = append(finalList, each)
			}
		}
		values = finalList
	}

	if len(attrs) != len(values) {
		return nil, fmt.Errorf("number of values (%d) does not match number of attributes (%d)", len(values), len(attrs))
	}

	attrDict := make(map[string]string)
	for i := range attrs {
		attrDict[attrs[i]] = values[i]
	}
	return attrDict, nil
}

// ConvertKeysToUpper converts map keys to uppercase
func ConvertKeysToUpper(dict map[string]interface{}) map[string]interface{} {
	result := make(map[string]interface{})
	for k, v := range dict {
		result[strings.ToUpper(k)] = v
	}
	return result
}

// IAConfigBuilder builds configuration for options like -I or -A
func IAConfigBuilder(cmdKey, optionKey string, configOptions map[string]string) (string, error) {
	if _, ok := HmcCmdOpt[cmdKey]; !ok {
		return "", fmt.Errorf("invalid command key: %s", cmdKey)
	}
	if _, ok := HmcCmdOpt[cmdKey][optionKey]; !ok {
		return "", fmt.Errorf("option %s not supported for command %s", optionKey, cmdKey)
	}
	attribute, ok := HmcCmdOpt[cmdKey][optionKey].(map[string]string)
	if !ok {
		return "", fmt.Errorf("%s for %s is not a map", optionKey, cmdKey)
	}

	configStr := fmt.Sprintf(" %s \"", strings.ToLower(optionKey))
	for key, value := range configOptions {
		if _, ok := attribute[key]; !ok {
			return "", fmt.Errorf("invalid attribute: %s", key)
		}
		var attr, addOrSubSign string
		if len(value) > 0 && value[0] == '-' {
			attr = attribute[key] + "-="
			addOrSubSign = "-"
		} else if len(value) > 0 && value[0] == '+' {
			attr = attribute[key] + "+="
			addOrSubSign = "+"
		} else {
			attr = attribute[key] + "="
			addOrSubSign = ""
		}
		if strings.Contains(value, ",") {
			if strings.Contains(value, "\"\"") {
				configStr += fmt.Sprintf("%s\\\"%s\\\"\",", attr, strings.TrimPrefix(value, addOrSubSign))
			} else {
				configStr += fmt.Sprintf("%s\"%s\",", attr, strings.TrimPrefix(value, addOrSubSign))
			}
		} else {
			configStr += fmt.Sprintf("%s%s,", attr, strings.TrimPrefix(value, addOrSubSign))
		}
	}
	configStr = strings.TrimSuffix(configStr, ",")
	configStr += "\""
	return configStr, nil
}
