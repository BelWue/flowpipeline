Segments in this group export flows to external databases. The distinction from the
output group lies in the fact that these exports are potentially lossy, i.e. some fields
might be lost. For instance, the `prometheus` segment as a metric provider does not
export any information about flow timing or duration, among others.
