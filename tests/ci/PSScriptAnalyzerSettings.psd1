@{
    # Gleiche Schweregrade wie der CI-Aufruf: nur Fehler/Warnungen melden.
    Severity = @('Error', 'Warning')

    # Write-Host ist hier gewuenscht: CLI-Ausgabe + ::notice::/::warning::/::error::.
    ExcludeRules = @(
        'PSAvoidUsingWriteHost'
    )
}
