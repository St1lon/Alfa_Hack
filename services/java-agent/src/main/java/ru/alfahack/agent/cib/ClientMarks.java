package ru.alfahack.agent.cib;

public record ClientMarks(
        boolean rko,
        boolean gar,
        boolean loa,
        boolean lea,
        boolean dep,
        boolean zpp,
        boolean factoring) {

    public boolean byMark(String mark) {
        if (mark == null) {
            return false;
        }
        return switch (mark) {
            case "CL_PRODUCT_RKO_MARK" -> rko;
            case "CL_PRODUCT_GAR_MARK" -> gar;
            case "CL_PRODUCT_LOA_MARK" -> loa;
            case "CL_PRODUCT_LEA_MARK" -> lea;
            case "CL_PRODUCT_DEP_MARK" -> dep;
            case "CL_PRODUCT_ZPP_MARK" -> zpp;
            case "FACTORING_MARK" -> factoring;
            default -> false;
        };
    }

    public String describe(String mark) {
        return mark + "=" + (byMark(mark) ? "1" : "0");
    }
}
