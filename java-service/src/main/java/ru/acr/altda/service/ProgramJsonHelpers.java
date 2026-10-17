package ru.acr.altda.service;

import com.fasterxml.jackson.databind.JsonNode;
import com.fasterxml.jackson.databind.ObjectMapper;
import com.fasterxml.jackson.databind.node.ObjectNode;
import org.springframework.stereotype.Component;

@Component
public class ProgramJsonHelpers {

    private final ObjectMapper objectMapper;

    public ProgramJsonHelpers(ObjectMapper objectMapper) {
        this.objectMapper = objectMapper;
    }

    public JsonNode wrapVariants(JsonNode variants) {
        if (variants != null && variants.isObject() && variants.has("variants")) {
            return variants;
        }
        ObjectNode wrapper = objectMapper.createObjectNode();
        wrapper.set("variants", variants == null ? objectMapper.createArrayNode() : variants);
        return wrapper;
    }

    public static JsonNode firstVariant(JsonNode variants) {
        JsonNode list = variantList(variants);
        if (list != null && list.isArray() && !list.isEmpty()) {
            return list.get(0);
        }
        return null;
    }

    public static JsonNode findVariant(JsonNode skeletonVariants, String key) {
        JsonNode list = variantList(skeletonVariants);
        if (list == null || !list.isArray()) {
            return null;
        }
        for (JsonNode v : list) {
            if (key.equals(v.path("key").asText())) {
                return v;
            }
        }
        return null;
    }

    private static JsonNode variantList(JsonNode variants) {
        if (variants != null && variants.isObject() && variants.has("variants")) {
            return variants.get("variants");
        }
        return variants;
    }
}
