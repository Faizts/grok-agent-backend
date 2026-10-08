# Social video publishing
Use for YouTube, Facebook Pages and Instagram. Read the entire workflow before transferring or publishing files.

## Scope and review
Generating a video does not authorize posting. Ask which platforms, exact YouTube channel/Facebook Page/Instagram account, title/caption/description, thumbnail, visibility, audience, and schedule with timezone when missing. Inspect signed-in account identity and selected destination; do not assume a personal profile is the requested Page. Reuse explicitly confirmed preferences.
Prepare a publish-plan.json containing local file, destination identity, title/caption, audience/disclosures, visibility and schedule. Show the user the finished video and metadata before the final posting action. If the user already explicitly approved this exact asset, destination and public/scheduled action, proceed without asking again. Otherwise request approval of the concrete plan before Publish/Share/Schedule. Tool always_allow does not substitute for that publishing instruction. Never reveal credentials or ask for passwords/OTPs; sign-in and 2FA are manual in Shared Computer.

## Platform workflows
Use current UI evidence instead of hardcoded selectors/limits. Verify file duration/aspect/size against the upload UI; make a separate export if needed without destroying the original. Do not silently crop meaningful content.
- YouTube: open Studio, verify channel, create/upload video using browser upload on the file input. Enter reviewed metadata and thumbnail, choose the user's audience setting and accurate paid-promotion/altered-content disclosures when applicable. Complete upload/checks. Keep private/draft if public/scheduled publication is not approved. Review visibility and timezone immediately before the final action.
- Facebook Page: open the selected Page's publishing tools or Meta Business Suite, verify Page and access. Create the requested video/post/reel, upload approved media, enter caption, preview destination and schedule. Avoid crossposting to additional accounts unless requested.
- Instagram: verify the exact signed-in profile or connected account in Meta Business Suite. Use the site's supported create/upload flow. Inspect crop/cover/caption and whether the current account supports requested scheduling. If unavailable, report the limitation and prepare files/metadata for manual posting; do not invent an API capability.

## Completion and recovery
Before retrying an ambiguous Publish result, inspect the content list and publication status to avoid duplicate posts. Record platform, destination, asset hash, status, URL/post ID and publication/schedule time in publish-result.json. Verify saved draft or public/scheduled state in the site and return the actual URL. An upload/processing spinner is not publication success. Stop if destination identity changes. Preserve a prepared upload if login/checks require user action.
References: https://support.google.com/youtube/answer/57407 ; https://www.facebook.com/help/289207354498410 (may require sign-in). Check the site's current guidance for limits and account eligibility.
