// fingerprintOf captures "what recipient source was actually submitted" — a
// file's identity (name+size+lastModified, a client-cheap proxy for content
// identity — reading the whole file to hash it is not worth it here) when a
// file is chosen, the exact pasted text otherwise. Used to detect whenever a
// reachability preview no longer matches the CURRENT input (CAM-09):
// changing pasted text or picking/replacing/clearing a file must
// immediately stop Create/Save from trusting a stale, no-longer-accurate
// preview — a campaign must never save an audience the operator never
// actually reviewed.
export function fingerprintOf(text: string, file: File | null | undefined): string {
  return file ? `file:${file.name}:${file.size}:${file.lastModified}` : `text:${text}`
}
