package ru.alfahack.agent.agent.tools;

import org.springframework.stereotype.Component;

import ru.alfahack.agent.agent.AgentRunContext;
import ru.alfahack.agent.agent.AgentTool;

@Component
public class GetCompanyProfileTool implements AgentTool {

    @Override
    public String name() {
        return "get_company_profile";
    }

    @Override
    public String run(AgentRunContext ctx) {
        var company = ctx.company();
        StringBuilder sb = new StringBuilder(512);
        sb.append("{\"company_id\":\"").append(company.companyId()).append('"');
        if (company.displayAlias() != null) {
            sb.append(",\"alias\":\"").append(company.displayAlias()).append('"');
        }
        sb.append(",\"industry\":\"").append(company.okvedName()).append('"');
        sb.append(",\"okved\":\"").append(company.okvedCode()).append('"');
        sb.append(",\"region\":\"").append(company.region()).append('"');
        sb.append(",\"revenue\":").append(company.revenue());
        sb.append(",\"workers\":\"").append(company.workersGradation()).append('"');
        sb.append(",\"stage\":\"").append(company.ecoMark()).append('"');
        sb.append(",\"client_type\":\"").append(company.clType()).append('"');
        sb.append(",\"fts_flag\":").append(company.ftsFlag());
        if (!company.ftsCountries().isEmpty()) {
            sb.append(",\"fts_countries\":[");
            for (int i = 0; i < company.ftsCountries().size(); i++) {
                if (i > 0) {
                    sb.append(',');
                }
                sb.append('"').append(company.ftsCountries().get(i)).append('"');
            }
            sb.append(']');
        }
        sb.append(",\"products\":{");
        sb.append("\"loa\":").append(company.marks().loa())
                .append(",\"lea\":").append(company.marks().lea())
                .append(",\"gar\":").append(company.marks().gar())
                .append(",\"factoring\":").append(company.marks().factoring())
                .append(",\"dep\":").append(company.marks().dep())
                .append(",\"zpp\":").append(company.marks().zpp());
        sb.append("}}");
        return sb.toString();
    }
}
