#!/usr/bin/env python3
"""Generate the Redex offense ATT&CK Navigator layer.

This is the *offense* counterpart to detections/attack/artex_navigator_layer.json
(which maps detection rules). Edit COVERAGE below when a capability is added or
removed; the generated JSON is committed for Navigator import.
"""
import json
import sys

# (technique ID, tactic, comment)
COVERAGE = [
    # Reconnaissance
    ("T1595", "reconnaissance", "nmap / masscan active scanning"),
    ("T1595.002", "reconnaissance", "ffuf / gobuster / wfuzz content discovery"),
    ("T1592", "reconnaissance", "whatweb fingerprinting"),
    ("T1590", "reconnaissance", "subfinder / dnsutils"),
    # Initial Access
    ("T1190", "initial-access", "sqlmap / nuclei / commix web exploitation"),
    ("T1078", "initial-access", "valid accounts via hydra / kerbrute / creds"),
    # Execution
    ("T1059", "execution", "Bash / shell_open"),
    ("T1047", "execution", "impacket-wmiexec"),
    ("T1053.005", "execution", "impacket-atexec"),
    # Persistence
    ("T1505.003", "persistence", "web shell written to target (manual)"),
    ("T1558.001", "persistence", "impacket-ticketer golden ticket"),
    # Privilege Escalation
    ("T1548", "privilege-escalation", "linpeas / LES SUID-sudo review"),
    ("T1068", "privilege-escalation", "kernel exploit chains"),
    ("T1644", "privilege-escalation", "certipy ADCS ESC abuse"),
    # Credential Access
    ("T1003.001", "credential-access", "pypykatz LSASS parsing"),
    ("T1003.003", "credential-access", "impacket-secretsdump NTDS"),
    ("T1110", "credential-access", "hydra / kerbrute spraying"),
    ("T1558.003", "credential-access", "impacket-GetUserSPNs kerberoast"),
    ("T1557.001", "credential-access", "mitm6 + impacket-ntlmrelayx"),
    ("T1555", "credential-access", "LaZagne credential stores"),
    # Discovery
    ("T1087", "discovery", "rpcclient / ldapsearch enumeration"),
    ("T1018", "discovery", "nmap host discovery"),
    ("T1069.002", "discovery", "bloodhound / ldapdomaindump"),
    # Lateral Movement
    ("T1021.002", "lateral-movement", "impacket-psexec/smbexec"),
    ("T1021.004", "lateral-movement", "ssh pivot"),
    ("T1550.002", "lateral-movement", "pass-the-hash"),
    # Command and Control
    ("T1071", "command-and-control", "sliver C2 / HTTPS channels"),
    ("T1095", "command-and-control", "sliver non-application protocol"),
    ("T1573", "command-and-control", "sliver encrypted channel"),
    ("T1570", "command-and-control", "lateral tool transfer / chisel, ligolo"),
    # Exfiltration
    ("T1567.002", "exfiltration", "rclone to cloud storage"),
    ("T1048", "exfiltration", "alternative protocol exfil test"),
]

TACTIC_ORDER = [
    "reconnaissance", "resource-development", "initial-access", "execution",
    "persistence", "privilege-escalation", "defense-evasion", "credential-access",
    "discovery", "lateral-movement", "collection", "command-and-control",
    "exfiltration", "impact",
]


def main():
    techniques = []
    for tid, tactic, comment in COVERAGE:
        techniques.append({
            "techniqueID": tid,
            "tactic": tactic,
            "score": 100,
            "color": "#e11d2e",
            "comment": comment,
            "enabled": True,
        })
    layer = {
        "name": "Redex offense coverage",
        "versions": {"attack": "16", "navigator": "4.9.0", "layer": "4.5"},
        "domain": "enterprise-attack",
        "description": "ATT&CK techniques Redex can execute from its shipped toolset.",
        "sorting": 0,
        "layout": {"layout": "side", "showID": True, "showName": True},
        "hideDisabled": False,
        "techniques": techniques,
        "gradient": {"colors": ["#ffe3e6", "#e11d2e"], "minValue": 0, "maxValue": 100},
        "legendItems": [
            {"label": "executable via shipped tools", "color": "#e11d2e"},
        ],
        "showTacticRowBackground": True,
        "tacticRowBackground": "#0b0b0f",
        "selectTechniquesAcrossTactics": True,
    }
    json.dump(layer, sys.stdout, indent=2, ensure_ascii=False)
    sys.stdout.write("\n")


if __name__ == "__main__":
    main()
