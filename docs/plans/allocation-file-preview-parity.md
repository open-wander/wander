# Allocation File Preview Parity

## Baseline evidence

The approved Nomad 1.6.5 UI behavior distinguishes allocation files by their
reported content type:

- images render as images;
- text and JSON render as text;
- unknown binary types show an unsupported state and remain downloadable;
- text files larger than 50 KB expose finite **Head** and **Tail** views;
- downloads return the complete file, independently of preview limits;
- growing text files can be streamed in a separate mode.

This is behavioral evidence only. Wander's handlers and templates remain an
independent implementation.

## Current unit

- [x] Stream complete downloads instead of reusing the bounded preview read.
- [x] Render allowlisted raster images through a same-origin, `nosniff`
      endpoint.
- [x] Keep unsupported binary files truthful and downloadable.
- [x] Restore 50 KB Head and Tail views for large text files.
- [x] Preserve namespace, region, and ACL token scope on every file read.
- [x] Cover empty and large files, image and unknown binary behavior, Head and
      Tail byte ranges, and full downloads.
- [x] Restore a bounded live stream for growing text files from the latest
      50 KB, with explicit start and stop behavior.

## Follow-on units

- [x] Restore separate allocation-level `/ui/allocations/{id}/fs` and
      task-level `/ui/allocations/{id}/{task}/fs` routes, including inherited
      task path prefixing and real-agent browser coverage.
- [x] Preserve special characters and reject parent traversal before an API
      read.
- [x] Preserve truthful status for vanished files, directories requested as
      files, permission failures, and mid-stream read failures.
