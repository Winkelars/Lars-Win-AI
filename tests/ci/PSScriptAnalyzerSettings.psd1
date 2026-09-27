@{
    # Gleiche Schweregrade wie der CI-Aufruf: nur Fehler/Warnungen melden.
    Severity = @('Error', 'Warning')

    # Write-Host ist hier gewuenscht: CLI-Ausgabe + ::notice::/::warning::/::error::.
    # Write-Log ueberschreibt auf manchen Runnern einen gleichnamigen Cmdlet-Namen,
    # ist aber durch docs/CONTRACTS.md §4 vorgeschrieben.
    ExcludeRules = @(
        'PSAvoidUsingWriteHost'
        'PSAvoidOverwritingBuiltInCmdlets'
    )
}
