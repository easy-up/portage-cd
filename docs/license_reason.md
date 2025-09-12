# License Decision Document

## Overview

This document explains the rationale behind the choice of the software license for this repository, which builds a security pipeline tool incorporating several open source components: Semgrep, Grype, ClamAV, Gitleaks, and Syft. It clarifies the licensing requirements of these components and guides users on important licensing considerations when using this repository.

***

## Chosen License: GNU General Public License version 2 (GPL v2)

The entire project is licensed under the **GNU GPL v2** license. This choice was made to ensure full compliance with the most restrictive licenses among the integrated components and to promote an open source ecosystem where derivative works remain free and open.

### Why GPL v2?

- **Copyleft Compliance:** ClamAV is licensed under GPL v2 with strong copyleft provisions. If ClamAV code is integrated or distributed as part of this tool, the entire combined work must also be licensed under GPL v2.
- **Compatibility:** GPL v2 is compatible with LGPL 2.1 (used by the Semgrep engine) and permissive licenses like Apache 2.0 (Grype, Syft) and MIT (Gitleaks), allowing these tools to be legally combined in a GPL v2 project.
- **Source Availability:** GPL v2 requires the distribution of corresponding source code, supporting transparency and enabling community contributions.
- **Avoiding License Conflicts:** Choosing GPL v2 simplifies compliance by aligning all code under a common copyleft framework that respects the strictest license requirements.

***

## Summary of Included Tools and Their Licenses

| Tool           | License                | Key License Points                                     | Notes for Users                                   |
|----------------|------------------------|-------------------------------------------------------|--------------------------------------------------|
| **Semgrep Engine**  | LGPL 2.1                | Allows commercial and LGPL-compliant use, modification, and distribution | Included in compliance with LGPL terms            |
| **Semgrep Rules**   | Semgrep Rules License v1.0 | Usage restricted to internal business; no redistribution or resale allowed | **Not redistributed with this repo**; users must source official rules independently |
| **Grype**          | Apache 2.0              | Permissive license allowing commercial use with attribution | Included fully under Apache 2.0 terms             |
| **Syft**           | Apache 2.0              | Same as Grype                                           | Included fully under Apache 2.0 terms             |
| **ClamAV**         | GPL v2                  | Strong copyleft; GPL v2 required if distributed with modifications or linked | Makes GPL v2 the controlling license for this repo |
| **Gitleaks**       | MIT                     | Permissive license; commercial use allowed with attribution | Included fully under MIT license                   |

***

## Important Licensing Notes for Users

- **Semgrep Official Rules**: The official Semgrep rules are **not included** in this repository or Docker container. They are governed by a separate Semgrep Rules License v1.0, which restricts redistribution. Users must download or manage these rules independently for internal use only.
- **Redistribution and Use**: Any redistribution or modification of this repository and its content must comply with GPL v2, including providing source code and maintaining license notices.
- **Integration of Tools**: Usage of integrated tools must respect their individual licenses, especially when incorporating or modifying their source code.
- **Custom Rules**: Users are encouraged to create and maintain their own Semgrep rules under compatible licenses, which can be freely distributed as part of this project.

***

## Summary

By adopting GPL v2, this project respects the licensing requirements of all combined tools—particularly ClamAV's GPL v2 copyleft. This approach safeguards your rights and those of the community by ensuring continued open source availability and legal clarity.

Users and contributors should familiarize themselves with the individual tool licenses detailed above to ensure proper use and compliance when using or extending this project.

***

For any legal questions or clarifications, please consult a qualified legal expert or open source licensing specialist.

***

*This decision document is based on up-to-date license terms of Semgrep, Grype, Syft, ClamAV, and Gitleaks as of August 2025.*

[1](https://ospo.cc.gatech.edu/releasing-open-source/)
[2](https://opensource.guide/legal/)
[3](https://www.civictheme.io/how-to-use-civictheme/open-source-licensing)
[4](https://www.reddit.com/r/opensource/comments/1eb9j8d/choosing_an_open_source_license_for_a_commercial/)
[5](https://github.com/joelparkerhenderson/decision-record)
[6](https://opensource.org/licenses)
[7](https://choosealicense.com)
[8](https://opensource.org/license/mit)