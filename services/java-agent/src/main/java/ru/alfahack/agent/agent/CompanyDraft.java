package ru.alfahack.agent.agent;

import java.util.List;

public record CompanyDraft(
        SecondOrder secondOrder,
        Impact impact,
        List<String> riskTags,
        Playbook playbook) {

    public record SecondOrder(String text, List<String> citations) {

        public SecondOrder {
            citations = citations == null ? List.of() : List.copyOf(citations);
        }
    }

    public record Impact(
            String type,
            String severity,
            Integer impactScore,
            Double probability,
            String timeHorizon,
            List<String> targetPersonas) {

        public Impact {
            targetPersonas = targetPersonas == null ? List.of() : List.copyOf(targetPersonas);
        }
    }

    public record Playbook(
            List<Action> whatToDo,
            List<String> whatToWatch,
            List<String> risksToConsider,
            List<String> questionsForTeam) {

        public Playbook {
            whatToDo = whatToDo == null ? List.of() : List.copyOf(whatToDo);
            whatToWatch = whatToWatch == null ? List.of() : List.copyOf(whatToWatch);
            risksToConsider = risksToConsider == null ? List.of() : List.copyOf(risksToConsider);
            questionsForTeam = questionsForTeam == null ? List.of() : List.copyOf(questionsForTeam);
        }
    }

    public record Action(String text, List<String> citations) {

        public Action {
            citations = citations == null ? List.of() : List.copyOf(citations);
        }
    }
}
