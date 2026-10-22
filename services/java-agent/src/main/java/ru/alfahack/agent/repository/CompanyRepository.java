package ru.alfahack.agent.repository;

import java.util.ArrayList;
import java.util.Arrays;
import java.util.LinkedHashMap;
import java.util.List;
import java.util.Map;

import org.springframework.jdbc.core.namedparam.MapSqlParameterSource;
import org.springframework.jdbc.core.namedparam.NamedParameterJdbcTemplate;
import org.springframework.stereotype.Repository;

import ru.alfahack.agent.agent.AgentRunContext;
import ru.alfahack.agent.cib.ClientMarks;

@Repository
public class CompanyRepository {

    private final NamedParameterJdbcTemplate jdbc;

    public CompanyRepository(NamedParameterJdbcTemplate jdbc) {
        this.jdbc = jdbc;
    }

    public AgentRunContext.CompanyBundle loadBundle(String companyId) {
        var row = jdbc.queryForObject("""
                SELECT company_id, display_alias, org_type, okfs_name,
                       company_age_years_gradation, eco_mark_label, mainokved_code, mainokved_name,
                       amount_workers_fns_gradation, cl_type, revenue_company, group_id,
                       city, region, fts_flag, fts_country_name,
                       cl_product_rko_mark, cl_product_gar_mark, cl_product_loa_mark,
                       cl_product_lea_mark, cl_product_dep_mark, cl_product_zpp_mark, factoring_mark
                FROM companies WHERE company_id = :id
                """, new MapSqlParameterSource("id", companyId), (rs, i) -> new Object[] {
                rs.getString("company_id"), rs.getString("display_alias"), rs.getString("org_type"),
                rs.getString("okfs_name"), rs.getString("company_age_years_gradation"),
                rs.getString("eco_mark_label"), rs.getString("mainokved_code"),
                rs.getString("mainokved_name"), rs.getString("amount_workers_fns_gradation"),
                rs.getString("cl_type"), (Double) rs.getObject("revenue_company"),
                rs.getString("group_id"), rs.getString("city"), rs.getString("region"),
                (Boolean) rs.getObject("fts_flag"), rs.getString("fts_country_name"),
                rs.getBoolean("cl_product_rko_mark"), rs.getBoolean("cl_product_gar_mark"),
                rs.getBoolean("cl_product_loa_mark"), rs.getBoolean("cl_product_lea_mark"),
                rs.getBoolean("cl_product_dep_mark"), rs.getBoolean("cl_product_zpp_mark"),
                rs.getBoolean("factoring_mark") });

        List<String> ftsCountries = row[15] == null || row[15].toString().isBlank()
                ? List.of()
                : Arrays.stream(row[15].toString().split(",")).map(String::strip).filter(s -> !s.isEmpty()).toList();

        ClientMarks marks = new ClientMarks(
                (Boolean) row[16], (Boolean) row[17], (Boolean) row[18],
                (Boolean) row[19], (Boolean) row[20], (Boolean) row[21], (Boolean) row[22]);

        Map<String, AgentRunContext.CompanyBundle.Feature> features = new LinkedHashMap<>();
        jdbc.query("""
                SELECT feature_key, feature_value::text AS value, derived_from
                FROM company_features WHERE company_id = :id
                """, new MapSqlParameterSource("id", companyId), (rs, i) -> {
            features.put(rs.getString("feature_key"), new AgentRunContext.CompanyBundle.Feature(
                    rs.getString("feature_key"), rs.getString("value"), rs.getString("derived_from"),
                    shortValue(rs.getString("value"))));
            return null;
        });
        jdbc.query("""
                SELECT feature_key, feature_value::text AS value
                FROM company_overrides WHERE company_id = :id
                """, new MapSqlParameterSource("id", companyId), (rs, i) -> {
            features.put(rs.getString("feature_key"), new AgentRunContext.CompanyBundle.Feature(
                    rs.getString("feature_key"), rs.getString("value"), "company_overrides",
                    shortValue(rs.getString("value"))));
            return null;
        });

        return new AgentRunContext.CompanyBundle(
                (String) row[0], (String) row[1], (String) row[2], (String) row[3], (String) row[4],
                (String) row[5], (String) row[6], (String) row[7], (String) row[8], (String) row[9],
                (Double) row[10], (String) row[11], (String) row[12], (String) row[13],
                (Boolean) row[14], ftsCountries, marks, List.copyOf(features.values()));
    }

    public List<AgentRunContext.Counterparty> loadCounterparties(String companyId, int limitPerDirection) {
        List<AgentRunContext.Counterparty> result = new ArrayList<>();
        collect(companyId, "payer", "OUT", limitPerDirection, result);
        collect(companyId, "beneficiary", "IN", limitPerDirection, result);
        return result;
    }

    private void collect(String companyId, String column, String direction, int limit, List<AgentRunContext.Counterparty> out) {
        jdbc.query("""
                SELECT e.%s AS other, c.mainokved_name AS industry, e.pay_amt,
                       100.0 * e.pay_amt / NULLIF(SUM(e.pay_amt) OVER (), 0) AS share_pct
                FROM counterparty_edges e
                LEFT JOIN companies c ON c.company_id = e.%s
                WHERE e.%s = :company
                ORDER BY e.pay_amt DESC
                LIMIT :limit
                """.formatted(column, opposite(column), column),
                new MapSqlParameterSource()
                        .addValue("company", companyId)
                        .addValue("limit", limit),
                (rs, i) -> {
                    out.add(new AgentRunContext.Counterparty(
                            rs.getString("other"), rs.getString("industry"), direction,
                            rs.getDouble("share_pct"), rs.getDouble("pay_amt")));
                    return null;
                });
    }

    private static String opposite(String column) {
        return "payer".equals(column) ? "beneficiary" : "payer";
    }

    public List<AgentRunContext.GroupMember> loadGroupMembers(String groupId, String excludeCompanyId, int limit) {
        if (groupId == null || groupId.isBlank()) {
            return List.of();
        }
        return jdbc.query("""
                SELECT company_id, region, mainokved_name FROM companies
                WHERE group_id = :gid AND company_id <> :exclude
                LIMIT :limit
                """, new MapSqlParameterSource()
                .addValue("gid", groupId)
                .addValue("exclude", excludeCompanyId)
                .addValue("limit", limit),
                (rs, i) -> new AgentRunContext.GroupMember(
                        rs.getString("company_id"), rs.getString("region"), rs.getString("mainokved_name")));
    }

    private static String shortValue(String json) {
        if (json == null) {
            return "";
        }
        String stripped = json.strip();
        return stripped.length() <= 60 ? stripped : stripped.substring(0, 60);
    }
}
